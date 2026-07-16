// Command analytics-server will become the single binary described in
// server_implementation_plan.md section 4 — registration, ingestion,
// dashboard, and admin CLI subcommands. Phase S0 only establishes the
// module and build; those subcommands land in Phase S1+.
package main

import "fmt"

func main() {
	fmt.Println("daliys-analytics-server: Phase S0 scaffold, no subcommands yet")
}
