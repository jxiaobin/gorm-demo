package main

import (
	"fmt"
	"encoding/binary"
	"net"
)

func main() {
	
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, 173218814)
	fmt.Printf("%v\n", ip)

        fmt.Printf("%v\n", binary.BigEndian.Uint32(ip))
}

