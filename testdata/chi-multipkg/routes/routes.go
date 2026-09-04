package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/syst3mctl/godoclive/testdata/chi-multipkg/handlers"
)

// Register registers on the router it is handed, and fans out to a
// per-resource registrar inside a prefixed Route block.
func Register(r chi.Router) {
	r.Get("/health", handlers.Health)

	r.Route("/api/v1", func(r chi.Router) {
		registerPayments(r)
	})
}

func registerPayments(r chi.Router) {
	r.Get("/payments", handlers.ListPayments)
	r.Post("/payments", handlers.CreatePayment)
	r.Get("/payments/{paymentID}", handlers.GetPayment)
}
