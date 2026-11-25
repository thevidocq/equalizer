// Command source-summary prints the summary the source reader would send
// for a folder, as JSON, without a panel: the lab's efficacy study gives it
// to the discovery as the panel would. Usage: source-summary <site folder>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/thevidocq/equalizer/source-reader/internal/extract"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: source-summary <site folder>")
		os.Exit(2)
	}
	s, err := extract.Read(os.Args[1], time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", " ")
	if err := e.Encode(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
