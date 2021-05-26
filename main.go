// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modSetupapi = windows.NewLazyDLL("setupapi.dll")

	procSetupDiGetDeviceInterfaceDetailW = modSetupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiGetClassDevsW             = modSetupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces      = modSetupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiEnumDeviceInfo            = modSetupapi.NewProc("SetupDiEnumDeviceInfo")
	procSetupDiDestroyDeviceInfoList     = modSetupapi.NewProc("SetupDiDestroyDeviceInfoList")
)

type HDEVINFO uintptr
type HWND uintptr

const (
	invalidHDEVINFO = ^HDEVINFO(0)
)

const (
	DIGCF_PRESENT         = 0x2
	DIGCF_ALLCLASSES      = 0x4
	DIGCF_DEVICEINTERFACE = 0x10
)

const (
	GUID_DEVINTERFACE_DISK   = "{53F56307-B6BF-11D0-94F2-00A0C91EFB8B}"
	GUID_DEVINTERFACE_VOLUME = "{53F5630D-B6BF-11D0-94F2-00A0C91EFB8B}"
)

type SP_DEVINFO_DATA struct {
	cbSize    uint32
	ClassGuid windows.GUID
	DevInst   uint32
	Reserved  uintptr
}

type SP_DEVICE_INTERFACE_DATA struct {
	cbSize             uint32
	InterfaceClassGuid windows.GUID
	Flags              uint32
	Reserved           uintptr
}

type SP_DEVICE_INTERFACE_DETAIL_DATA struct {
	cbSize     uint32 // should be set to 6 on 386, 8 on amd64
	DevicePath [1]uint16
}

func FindDevices(classGUID windows.GUID) ([]string, error) {
	dis, err := SetupDiGetClassDevs(&classGUID, nil, 0, DIGCF_PRESENT|DIGCF_DEVICEINTERFACE)
	if err != nil {
		return nil, err
	}
	defer SetupDiDestroyDeviceInfoList(dis)

	var idata SP_DEVINFO_DATA
	idata.cbSize = uint32(unsafe.Sizeof(idata))

	var edata SP_DEVICE_INTERFACE_DATA
	edata.cbSize = uint32(unsafe.Sizeof(edata))

	var v []string
	for i := uint32(0); SetupDiEnumDeviceInfo(dis, i, &idata) == nil; i++ {
		for j := uint32(0); SetupDiEnumDeviceInterfaces(dis, &idata, &classGUID, j, &edata) == nil; j++ {

			p, err := getDevicePath(dis, &edata)
			if err != nil {
				return nil, fmt.Errorf("GetDevicePath: %v", err)
			}

			v = append(v, p)
		}
	}
	return v, nil
}

func SetupDiGetClassDevs(classGuid *windows.GUID, enumerator *uint16, hwndParent HWND, flags uint32) (handle HDEVINFO, err error) {
	r0, _, e1 := syscall.Syscall6(procSetupDiGetClassDevsW.Addr(), 4, uintptr(unsafe.Pointer(classGuid)), uintptr(unsafe.Pointer(enumerator)), uintptr(hwndParent), uintptr(flags), 0, 0)
	handle = HDEVINFO(r0)
	if handle == invalidHDEVINFO {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func SetupDiEnumDeviceInterfaces(devInfoSet HDEVINFO, devInfoData *SP_DEVINFO_DATA, intfClassGuid *windows.GUID, memberIndex uint32, devIntfData *SP_DEVICE_INTERFACE_DATA) (err error) {
	r1, _, e1 := syscall.Syscall6(procSetupDiEnumDeviceInterfaces.Addr(), 5, uintptr(devInfoSet), uintptr(unsafe.Pointer(devInfoData)), uintptr(unsafe.Pointer(intfClassGuid)), uintptr(memberIndex), uintptr(unsafe.Pointer(devIntfData)), 0)
	if r1 == 0 {
		err = error(e1)
	}
	return
}

func SetupDiEnumDeviceInfo(devInfoSet HDEVINFO, memberIndex uint32, devInfoData *SP_DEVINFO_DATA) (err error) {
	r1, _, e1 := syscall.Syscall(procSetupDiEnumDeviceInfo.Addr(), 3, uintptr(devInfoSet), uintptr(memberIndex), uintptr(unsafe.Pointer(devInfoData)))
	if r1 == 0 {
		err = error(e1)
	}
	return
}

func SetupDiDestroyDeviceInfoList(devInfoSet HDEVINFO) (err error) {
	r1, _, e1 := syscall.Syscall(procSetupDiDestroyDeviceInfoList.Addr(), 1, uintptr(devInfoSet), 0, 0)
	if r1 == 0 {
		err = error(e1)
	}
	return
}

func SetupDiGetDeviceInterfaceDetail(devInfoSet HDEVINFO, dintfdata *SP_DEVICE_INTERFACE_DATA, detail *SP_DEVICE_INTERFACE_DETAIL_DATA, detailSize uint32, reqsize *uint32, devInfData *SP_DEVINFO_DATA) (err error) {
	// func SetupDiGetDeviceInterfaceDetail(devInfoSet HDEVINFO, dintfdata *SP_DEVICE_INTERFACE_DATA, detail *SP_DEVICE_INTERFACE_DETAIL_DATA, detailSize uint32, reqsize *uint32, devInfData *SP_DEVINFO_DATA) (err error) {
	r1, _, e1 := syscall.Syscall6(procSetupDiGetDeviceInterfaceDetailW.Addr(), 6, uintptr(devInfoSet), uintptr(unsafe.Pointer(dintfdata)), uintptr(unsafe.Pointer(detail)), uintptr(detailSize), uintptr(unsafe.Pointer(reqsize)), uintptr(unsafe.Pointer(devInfData)))
	if r1 == 0 {
		err = error(e1)
	}
	return
}

func getDevicePath(dis HDEVINFO, edata *SP_DEVICE_INTERFACE_DATA) (path string, err error) {
	var (
		bufSize uint32
		cbSize  = uint32(unsafe.Sizeof(SP_DEVICE_INTERFACE_DETAIL_DATA{}))
	)

	// get required buffer size
	err = SetupDiGetDeviceInterfaceDetail(dis, edata, nil, 0, &bufSize, nil)
	if err != windows.ERROR_INSUFFICIENT_BUFFER {
		return
	}

	didd := &SP_DEVICE_INTERFACE_DETAIL_DATA{
		cbSize: cbSize,
	}

	err = SetupDiGetDeviceInterfaceDetail(dis, edata, didd, bufSize, nil, nil)
	if err != nil {
		return
	}

	// path = windows.UTF16PtrToString(&didd.DevicePath[0])
	path = ""

	return
}

func GUID(v string) (guid windows.GUID) {
	guid, _ = windows.GUIDFromString(v)
	return
}
