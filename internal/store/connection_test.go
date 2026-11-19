package store

import "testing"

func TestValidateProductionDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{name: "verified TLS", dsn: "postgres://arima@db.example.test:5432/arima?sslmode=verify-full"},
		{name: "TLS disabled", dsn: "postgres://arima@db.example.test:5432/arima?sslmode=disable", wantErr: true},
		{name: "prefer mode", dsn: "postgres://arima@db.example.test:5432/arima?sslmode=prefer", wantErr: true},
		{name: "missing TLS mode", dsn: "postgres://arima@db.example.test:5432/arima", wantErr: true},
		{name: "non URL DSN", dsn: "host=db.example.test user=arima sslmode=verify-full", wantErr: true},
		{name: "missing hostname", dsn: "postgres:///arima?sslmode=verify-full", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProductionDSN(tc.dsn)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateProductionDSN() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
