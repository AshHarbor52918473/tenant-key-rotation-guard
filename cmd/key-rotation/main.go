package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"example.com/infrai-key-rotation/rotation"
)

func main() {
	tenant := flag.String("tenant", "", "tenant project ID")
	name := flag.String("name", "tenant-api", "temporary key name")
	scopes := flag.String("scopes", "", "comma-separated key scopes")
	grace := flag.Int("grace-hours", 24, "old-key overlap window")
	change := flag.String("change-id", "", "unique maintenance change ID")
	flag.Parse()

	key := os.Getenv("INFRAI_API_KEY")
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = rotation.DefaultBaseURL
	}
	if key == "" || *tenant == "" || *change == "" || *grace < 1 {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY, -tenant, -change-id, and a positive -grace-hours are required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := rotation.NewClient(baseURL, key, nil)
	result, err := client.RotateTenantKey(ctx, rotation.RotationInput{
		TenantID: *tenant, Name: *name, Scopes: splitScopes(*scopes), GraceHours: *grace, IdempotencyID: *change,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

func splitScopes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if scope := strings.TrimSpace(part); scope != "" {
			result = append(result, scope)
		}
	}
	return result
}
