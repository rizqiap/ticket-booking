package auth

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRegisterInputValidation(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{"valid input", "user@example.com", "password123", false},
		{"invalid email", "invalid", "password123", true},
		{"short password", "user@example.com", "short", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RegisterInput{Name: "User", Email: tt.email, Password: tt.password}
			if err := binding.Validator.ValidateStruct(input); (err != nil) != tt.wantErr {
				t.Fatalf("validation error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
