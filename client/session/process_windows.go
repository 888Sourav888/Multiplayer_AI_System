//go:build windows

package session

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	rstrtmgr                = syscall.NewLazyDLL("rstrtmgr.dll")
	procRmStartSession      = rstrtmgr.NewProc("RmStartSession")
	procRmRegisterResources = rstrtmgr.NewProc("RmRegisterResources")
	procRmGetList           = rstrtmgr.NewProc("RmGetList")
	procRmEndSession        = rstrtmgr.NewProc("RmEndSession")
)

type RM_UNIQUE_PROCESS struct {
	dwProcessId    uint32
	dwLowDateTime  uint32
	dwHighDateTime uint32
}

type RM_PROCESS_INFO struct {
	Process             RM_UNIQUE_PROCESS
	strAppName          [256]uint16
	strServiceShortName [64]uint16
	ApplicationType     uint32
	AppStatus           uint32
	TSSessionId         uint32
	bRestartable        int32
}

func getProcessNameByPID(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	var buf [1024]uint16
	size := uint32(len(buf))
	err = windows.QueryFullProcessImageName(h, 0, &buf[0], &size)
	if err != nil {
		return "", err
	}
	return filepath.Base(windows.UTF16ToString(buf[:size])), nil
}

func findLockingProcesses(filePath string) ([]uint32, []string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		pids, names, err := queryLockingProcessesOnce(filePath)
		if err == nil && len(pids) > 0 {
			return pids, names, nil
		}
		if attempt < 2 {
			time.Sleep(2 * time.Millisecond)
		}
	}
	return nil, nil, nil
}

func queryLockingProcessesOnce(filePath string) ([]uint32, []string, error) {
	var sessionHandle uint32
	var sessionKey [33]uint16

	r, _, _ := procRmStartSession.Call(
		uintptr(unsafe.Pointer(&sessionHandle)),
		0,
		uintptr(unsafe.Pointer(&sessionKey[0])),
	)
	if r != 0 {
		return nil, nil, fmt.Errorf("RmStartSession failed: %d", r)
	}
	defer procRmEndSession.Call(uintptr(sessionHandle))

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	pathPtr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return nil, nil, err
	}

	fileNames := []*uint16{pathPtr}
	r, _, _ = procRmRegisterResources.Call(
		uintptr(sessionHandle),
		1,
		uintptr(unsafe.Pointer(&fileNames[0])),
		0,
		0,
		0,
		0,
	)
	if r != 0 {
		return nil, nil, fmt.Errorf("RmRegisterResources failed: %d", r)
	}

	var pnProcInfoNeeded uint32
	var pnProcInfo uint32
	var rebootReasons uint32

	// Query size first
	r, _, _ = procRmGetList.Call(
		uintptr(sessionHandle),
		uintptr(unsafe.Pointer(&pnProcInfoNeeded)),
		uintptr(unsafe.Pointer(&pnProcInfo)),
		0,
		uintptr(unsafe.Pointer(&rebootReasons)),
	)

	// 234 is ERROR_MORE_DATA
	if r != 0 && r != 234 {
		return nil, nil, fmt.Errorf("RmGetList size query failed: %d", r)
	}

	if pnProcInfoNeeded == 0 {
		return nil, nil, nil
	}

	procInfo := make([]RM_PROCESS_INFO, pnProcInfoNeeded)
	pnProcInfo = pnProcInfoNeeded

	r, _, _ = procRmGetList.Call(
		uintptr(sessionHandle),
		uintptr(unsafe.Pointer(&pnProcInfoNeeded)),
		uintptr(unsafe.Pointer(&pnProcInfo)),
		uintptr(unsafe.Pointer(&procInfo[0])),
		uintptr(unsafe.Pointer(&rebootReasons)),
	)
	if r != 0 {
		return nil, nil, fmt.Errorf("RmGetList retrieval failed: %d", r)
	}

	var pids []uint32
	var names []string
	for i := uint32(0); i < pnProcInfo; i++ {
		pid := procInfo[i].Process.dwProcessId
		name, err := getProcessNameByPID(pid)
		if err != nil {
			// fallback to app name in struct
			name = windows.UTF16ToString(procInfo[i].strAppName[:])
			if name == "" {
				name = "Unknown"
			}
		}
		pids = append(pids, pid)
		names = append(names, name)
	}

	return pids, names, nil
}

func findActiveAIProcesses(aiNames map[string]bool) (map[uint32]string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snapshot, &entry)
	if err != nil {
		return nil, err
	}

	results := make(map[uint32]string)
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if isAIProcessName(name, aiNames) {
			results[entry.ProcessID] = name
		}

		err = windows.Process32Next(snapshot, &entry)
		if err != nil {
			break
		}
	}

	return results, nil
}
