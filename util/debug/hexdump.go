package debug

import "fmt"

// Hexdump is a helper function that prints a hexdump of the given data.
func Hexdump(data []byte) {
	bytesPerLine := 16

	for i := 0; i < len(data); i += bytesPerLine {
		fmt.Printf("%04x: ", i)
		for j := 0; j < bytesPerLine; j++ {
			if i+j < len(data) {
				fmt.Printf("%02x ", data[i+j])
			} else {
				fmt.Printf("   ")
			}
		}
		fmt.Printf(" ")
		for j := 0; j < bytesPerLine; j++ {
			if i+j < len(data) {
				if data[i+j] >= 32 && data[i+j] < 127 {
					fmt.Printf("%c", data[i+j])
				} else {
					fmt.Printf(".")
				}
			}
		}
		fmt.Printf("\n")
	}
}
