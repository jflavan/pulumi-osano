// Package main runs the provider's gRPC server.
package main

import (
    "context"
    "fmt"
    "os"

    osano "github.com/highfiveghost/pulumi-osano/provider"
)

func main() {
    err := osano.Provider().Run(context.Background(), osano.Name, osano.Version)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
        os.Exit(1)
    }
}
