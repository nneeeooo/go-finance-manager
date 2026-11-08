package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
)

func TestLoanLifecycle(t *testing.T) {
	user, other := createTestUser(t), createTestUser(t)

	var id string
	apitest.New().Handler(router).Post("/api/v1/loans").Header("Authorization", authHeaderForUser(t, user)).
		JSON(`{"name":"Розстрочка","bank":1,"currency":980,"principal":11000000,"payment":667333,"last_payment":667341,
			"payments_total":24,"payments_paid":6,"first_due":"2026-06-30"}`).
		Expect(t).Status(http.StatusCreated).
		Assert(func(res *http.Response, _ *http.Request) error {
			body, _ := io.ReadAll(res.Body)
			var loan struct{ ID string }
			if err := json.Unmarshal(body, &loan); err != nil || loan.ID == "" {
				t.Fatalf("response: %s", body)
			}
			id = loan.ID
			return nil
		}).End()

	// Mark one more payment as made
	apitest.New().Handler(router).Patch("/api/v1/loans/"+id).Header("Authorization", authHeaderForUser(t, user)).
		JSON(`{"payments_paid":7}`).Expect(t).Status(http.StatusOK).
		Assert(func(res *http.Response, _ *http.Request) error {
			body, _ := io.ReadAll(res.Body)
			var loan struct {
				PaymentsPaid int `json:"payments_paid"`
			}
			_ = json.Unmarshal(body, &loan)
			if loan.PaymentsPaid != 7 {
				t.Errorf("paid: %s", body)
			}
			return nil
		}).End()

	// More paid than the total, a bad date, and somebody else's loan
	apitest.New().Handler(router).Patch("/api/v1/loans/"+id).Header("Authorization", authHeaderForUser(t, user)).
		JSON(`{"payments_paid":25}`).Expect(t).Status(http.StatusBadRequest).End()
	apitest.New().Handler(router).Post("/api/v1/loans").Header("Authorization", authHeaderForUser(t, user)).
		JSON(`{"name":"x","bank":1,"currency":980,"payment":100,"payments_total":3,"first_due":"06/2026"}`).
		Expect(t).Status(http.StatusBadRequest).End()
	apitest.New().Handler(router).Patch("/api/v1/loans/"+id).Header("Authorization", authHeaderForUser(t, other)).
		JSON(`{"payments_paid":1}`).Expect(t).Status(http.StatusNotFound).End()
	apitest.New().Handler(router).Get("/api/v1/loans").Header("Authorization", authHeaderForUser(t, other)).
		Expect(t).Status(http.StatusOK).Body(`[]`).End()

	apitest.New().Handler(router).Delete("/api/v1/loans/"+id).Header("Authorization", authHeaderForUser(t, user)).
		Expect(t).Status(http.StatusNoContent).End()
}
