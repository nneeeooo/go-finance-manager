package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"finance-api/internal/config"
	"finance-api/internal/sso"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSO routes (BFF): the backend runs the OIDC code flow with the central auth service; the browser only gets the
// app's own HttpOnly session cookie. They live at /auth/*, outside /api/v1, so the SPA and the auth service's
// redirect can reach them on the app's origin.

func ssoClient(c *gin.Context) *sso.Client { return c.MustGet("sso").(*sso.Client) }

// loginFailed sends the browser back to the app with an error code the UI shows
func loginFailed(c *gin.Context, code string) {
	c.Redirect(http.StatusFound, "/?auth_error="+url.QueryEscape(code))
}

// SSOLogin starts the login: state, nonce and PKCE verifier go into a signed short-lived cookie, the browser goes
// to the auth service. return_to must be a path on this site, else "/".
func SSOLogin(c *gin.Context) {
	cfg := c.MustGet("config").(*config.Config)
	client := ssoClient(c)

	state, err1 := sso.RandomToken()
	nonce, err2 := sso.RandomToken()
	verifier, err3 := sso.RandomToken()
	if err := errors.Join(err1, err2, err3); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start login"})
		return
	}
	returnTo := sso.SafeReturnTo(c.Query("return_to"))

	target, err := client.AuthCodeURL(c.Request.Context(), state, nonce, verifier, c.Query("provider"))
	if err != nil {
		log.Println("sso login:", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Auth service unavailable"})
		return
	}
	sealed, err := sso.LoginState{State: state, Nonce: nonce, Verifier: verifier, ReturnTo: returnTo}.Seal(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start login"})
		return
	}
	sso.SetCookie(c.Writer, cfg, sso.LoginCookie, sealed, "/auth", 600)
	c.Redirect(http.StatusFound, target)
}

// SSOCallback finishes the login: checks state against the cookie, exchanges the code server-side (PKCE),
// verifies the id_token (signature, issuer, audience, expiry, nonce), links or creates the user, creates the
// session and sets its cookie.
func SSOCallback(c *gin.Context) {
	cfg := c.MustGet("config").(*config.Config)
	db := c.MustGet("db").(*gorm.DB)
	client := ssoClient(c)

	cookie, _ := c.Cookie(sso.LoginCookie)
	sso.ClearCookie(c.Writer, cfg, sso.LoginCookie, "/auth") // single use, whatever happens next
	login, err := sso.OpenLoginState(cfg, cookie)
	if err == nil && !sso.StateEqual(c.Query("state"), login.State) {
		err = errors.New("state does not match the cookie")
	}
	if err != nil {
		// Several cookies of this name (other paths) would shadow the right one: worth seeing in the log
		n := 0
		for _, ck := range c.Request.Cookies() {
			if ck.Name == sso.LoginCookie {
				n++
			}
		}
		log.Printf("sso callback: invalid login state: %v (%s cookies: %d)", err, sso.LoginCookie, n)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login state"})
		return
	}

	if code := c.Query("error"); code != "" {
		loginFailed(c, safeErrorCode(code))
		return
	}
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code"})
		return
	}

	tok, identity, err := client.Exchange(c.Request.Context(), code, login.Verifier)
	if err != nil {
		log.Println("sso callback:", err)
		if errors.Is(err, sso.ErrUpstream) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Auth service unavailable"})
			return
		}
		loginFailed(c, "login_failed")
		return
	}
	if !sso.StateEqual(identity.Nonce, login.Nonce) {
		log.Println("sso callback: nonce mismatch")
		loginFailed(c, "login_failed")
		return
	}

	user, err := sso.ResolveUser(db, identity)
	if err != nil {
		if errors.Is(err, sso.ErrNotInvited) {
			loginFailed(c, "not_invited")
			return
		}
		log.Println("sso callback: user:", err)
		loginFailed(c, "login_failed")
		return
	}

	raw, err := sso.CreateSession(db, user.ID, tok)
	if err != nil {
		log.Println("sso callback: session:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}
	sso.SetCookie(c.Writer, cfg, sso.SessionCookie, raw, "/", int(sso.SessionTTL.Seconds()))
	c.Redirect(http.StatusFound, sso.SafeReturnTo(login.ReturnTo))
}

// SSOLogout ends the session: revokes the refresh token at the auth service, deletes the session row, clears the
// cookie. The browser is sent to the auth service's end_session URL (which ends its SSO session and comes back to
// AUTH_POST_LOGOUT_URL): as JSON {"redirect_url"} for fetch(), else as a 303.
func SSOLogout(c *gin.Context) {
	cfg := c.MustGet("config").(*config.Config)
	db := c.MustGet("db").(*gorm.DB)
	client := ssoClient(c)

	if !sso.CSRFOK(cfg, c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cross-site request rejected"})
		return
	}

	if raw, err := c.Cookie(sso.SessionCookie); err == nil && raw != "" {
		if s := sso.DeleteSession(db, raw); s != nil {
			if err := client.Revoke(c.Request.Context(), s.RefreshToken); err != nil {
				log.Println("sso logout: revoke:", err)
			}
		}
	}
	sso.ClearCookie(c.Writer, cfg, sso.SessionCookie, "/")

	target := client.EndSessionURL(c.Request.Context())
	if target == "" {
		target = cfg.AuthPostLogoutURL
	}
	if target == "" {
		target = "/"
	}
	if c.GetHeader("X-Requested-With") != "" || strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"redirect_url": target})
		return
	}
	c.Redirect(http.StatusSeeOther, target)
}

// safeErrorCode keeps an OAuth error code short and plain before it goes into a URL
func safeErrorCode(code string) string {
	if len(code) > 40 || strings.ContainsFunc(code, func(r rune) bool {
		return !(r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		return "login_failed"
	}
	return code
}
