package firestoredb

import (
	"net"
)

// checkIPState is a spport function that will take an IP address as an argument
// and run shell command `host` in the OS to verify if the IP and the respective
// dns name are matching (sign for a legit address)- will return `true` if it is!
// The method described [on developers.google.com](https://developers.google.com/search/docs/advanced/crawling/verifying-googlebot?visit_id=637824137217053907-3380645301&rd=1)
// func checkIPState(ip string) bool {
// 	// construct the first OS shell command
// 	cmd := exec.Command("host", ip)
// 	if output, err := cmd.Output(); err != nil {
// 		fmt.Printf("while running `host` for IP:%s, raised an error: %v\n", ip, err)
// 	} else {
// 		fmt.Printf("Output: %s\n", output)
// 		output_parts := strings.Split(string(output), " domain name pointer ")
// 		if len(output_parts) > 1 {
// 			fmt.Printf("the domain name pointer is: %s", output_parts[1])
// 			cmd2 := exec.Command("host", output_parts[1])
// 			if output2, err2 := cmd2.Output(); err2 != nil {
// 				fmt.Printf("while running `host` for dns: %s, error raised:%v", output_parts[1], err2)
// 			} else {
// 				output_parts2 := strings.Split(string(output2), " has address ")
// 				if len(output_parts2) > 1 {
// 					fmt.Printf("the reverse lookup returned IP: %s", output_parts2[1])
// 					if output_parts2[1] == ip {
// 						return true
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return false
// }

func nativeReverseDNSLookup(ip string) bool {

	addrs, err := net.LookupAddr(ip)
	if err != nil {
		// fmt.Printf("when lookup IP: %s, an error raised: %v\n", ip, err)
		return false
	}
	for _, addr := range addrs {
		ips, err := net.LookupIP(addr)
		if err != nil {
			// fmt.Printf("while lookup addr: %s, an error raised %v\n", addr, err)
			continue
		}
		for _, ipr := range ips {
			if ipr.String() == ip {
				return true
			}
		}
	}
	return false
}
