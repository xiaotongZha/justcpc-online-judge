// Command cleanup removes only disposable HTTP-test artifacts under .work.
// It deliberately preserves the running service binaries and Go build cache.
package main

import (
	"fmt"
	"os"
)

func main() {
	for _, path := range []string{
		"test/manual/.work/admin.cookie",
		"test/manual/.work/alice.cookie",
		"test/manual/.work/bob.cookie",
		"test/manual/.work/coach.cookie",
		"test/manual/.work/enqueue.json",
		"test/manual/.work/scoreboard.json",
		"test/manual/.work/contest-17-result.zip",
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
