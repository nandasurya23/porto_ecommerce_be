package modules

import "testing"

func TestComputeShippingCost(t *testing.T) {
	if got := computeShippingCost(100000); got != 20000 {
		t.Fatalf("expected shipping cost 20000, got %v", got)
	}
	if got := computeShippingCost(500000); got != 0 {
		t.Fatalf("expected free shipping, got %v", got)
	}
}

func TestNormalizePaymentMethod(t *testing.T) {
	if got := normalizePaymentMethod("qris_simulation"); got != "QRIS_SIMULATION" {
		t.Fatalf("expected QRIS_SIMULATION, got %s", got)
	}
	if got := normalizePaymentMethod("invalid"); got != "BANK_TRANSFER" {
		t.Fatalf("expected default BANK_TRANSFER, got %s", got)
	}
}
