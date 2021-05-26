package main

import (
	"fmt"
	"testing"
)

func TestGetDiskInfo(t *testing.T) {
	devs, err := FindDevices(GUID(GUID_DEVINTERFACE_DISK))
	if err != nil {
		t.Error(err)
		return
	}

	for _, dev := range devs {
		fmt.Println(dev)
	}
}
