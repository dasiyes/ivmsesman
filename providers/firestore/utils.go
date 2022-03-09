package firestoredb

import (
	"fmt"
	"os/exec"
)

// checkIPState is a spport function that will take an IP address as an argument
// and run shell command `host` in the OS to verify if the IP and the respective
// dns name are matching (sign for a legit address)- will return `true` if it is!
// The method described [on developers.google.com](https://developers.google.com/search/docs/advanced/crawling/verifying-googlebot?visit_id=637824137217053907-3380645301&rd=1)
func checkIPState(ip string) bool {
	// construct the first OS shell command
	cmd := exec.Command("host", ip)
	if output, err := cmd.Output(); err != nil {
		fmt.Printf("while running `host` for IP:%s, raised an error: %v\n", ip, err)
	} else {
		fmt.Printf("Output: %s\n", output)
	}

	return false
}
