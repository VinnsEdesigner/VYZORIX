// Command devpass prints an Argon2id hash for a password, using the API's own hasher.
//
// It exists so a developer can set a known password on a local database without
// reimplementing the hashing parameters (and getting them subtly wrong). It is a dev
// tool: it prints a hash, never writes a database, and refuses to run in production.
package main

import (
	"fmt"
	"os"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security/password"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: devpass <password>")
		os.Exit(2)
	}
	if os.Getenv("NODE_ENV") == "production" {
		fmt.Fprintln(os.Stderr, "devpass refuses to run with NODE_ENV=production")
		os.Exit(1)
	}

	hash, err := password.NewArgon2idHasher().Hash(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
