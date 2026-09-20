package main

import "testing"

func TestServiceName(t *testing.T) {
	if serviceName() != "API Service" {
		t.Fatalf("unexpected service name: %s", serviceName())
	}
}
