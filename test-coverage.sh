#!/bin/bash

echo "Running tests with coverage for internal packages only"

go test -coverprofile=coverage.out ./internal/...

echo ""
echo "Total coverage for internal packages:"
go tool cover -func=coverage.out | grep total

echo ""
echo "Coverage by package:"
go tool cover -func=coverage.out | grep -E "database|handlers|service" | awk '{print $1 ": " $NF}'

go tool cover -html=coverage.out -o coverage.html
echo ""
echo "HTML report generated: coverage.html"
