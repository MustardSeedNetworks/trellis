// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package capture

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// nativeWifiScanner reads the host adapter through the Native Wifi API in
// wlanapi.dll.
//
// Pure Go: the calls go through golang.org/x/sys/windows rather than cgo, so
// R5's cgo boundary costs Windows nothing (ADR-0006).
//
// Windows 11 gates WLAN scanning on Location Services, the same way macOS does
// and for the same reason — a visible BSS list locates the machine. Elevation
// does not substitute: measured on Windows 11 26200, a process running as
// nt authority\system with administrator rights still gets ERROR_ACCESS_DENIED
// from WlanScan while the location consent is denied, and netsh says so in as
// many words. Enable it under Privacy & security > Location.
//
// So this is a per-user consent in an interactive session, not a privilege —
// which is why linking capture into the user-session daemon (ADR-0006) is right
// on Windows for the same reason it is on macOS.
type nativeWifiScanner struct{}

var (
	wlanapi                   = windows.NewLazySystemDLL("wlanapi.dll")
	procWlanOpenHandle        = wlanapi.NewProc("WlanOpenHandle")
	procWlanCloseHandle       = wlanapi.NewProc("WlanCloseHandle")
	procWlanEnumInterfaces    = wlanapi.NewProc("WlanEnumInterfaces")
	procWlanScan              = wlanapi.NewProc("WlanScan")
	procWlanGetNetworkBssList = wlanapi.NewProc("WlanGetNetworkBssList")
	procWlanRegisterNotif     = wlanapi.NewProc("WlanRegisterNotification")
	procWlanFreeMemory        = wlanapi.NewProc("WlanFreeMemory")
)

const (
	// wlanClientVersion 2 is the Vista-and-later API. Version 1 is the XP
	// behaviour, which reports a different notification set.
	wlanClientVersion = 2

	// dot11BSSTypeAny asks for infrastructure and ad-hoc BSSs alike, so a
	// survey does not silently omit peer-to-peer networks sharing the channel.
	dot11BSSTypeAny = 3

	notificationSourceACM = 0x00000008
	acmScanComplete       = 7
	acmScanFail           = 8

	// kHzPerMHz converts WLAN_BSS_ENTRY's centre frequency, which Windows
	// reports in kHz where every other platform uses MHz.
	kHzPerMHz = 1000
)

// scanTimeout bounds the wait for a scan-complete notification. Microsoft
// documents the scan as finishing within four seconds; past this the radio is
// not coming back and reporting that beats blocking a survey.
const scanTimeout = 20 * time.Second

// dot11SSID mirrors DOT11_SSID.
type dot11SSID struct {
	Length uint32
	SSID   [32]byte
}

// wlanRateSet mirrors WLAN_RATE_SET.
type wlanRateSet struct {
	RateSetLength uint32
	RateSet       [126]uint16
}

// wlanBSSEntry mirrors WLAN_BSS_ENTRY. The blank fields are C's alignment
// padding, written out because Go would otherwise lay the struct out
// differently and every field after the first gap would be read from the wrong
// offset. TestWLANBSSEntryLayout pins them.
type wlanBSSEntry struct {
	SSID                  dot11SSID
	PhyID                 uint32
	BSSID                 [6]byte
	_                     [2]byte
	BSSType               uint32
	BSSPhyType            uint32
	RSSI                  int32
	LinkQuality           uint32
	InRegDomain           byte
	_                     [1]byte
	BeaconPeriod          uint16
	_                     [4]byte
	Timestamp             uint64
	HostTimestamp         uint64
	CapabilityInformation uint16
	_                     [2]byte
	ChCenterFrequency     uint32
	RateSet               wlanRateSet
	IeOffset              uint32
	IeSize                uint32
}

// wlanInterfaceInfo mirrors WLAN_INTERFACE_INFO.
type wlanInterfaceInfo struct {
	InterfaceGUID        windows.GUID
	InterfaceDescription [256]uint16
	State                uint32
}

// wlanNotificationData mirrors WLAN_NOTIFICATION_DATA.
type wlanNotificationData struct {
	NotificationSource uint32
	NotificationCode   uint32
	InterfaceGUID      windows.GUID
	DataSize           uint32
	Data               uintptr
}

// Waiters for scan-complete notifications, keyed by a token handed to the
// callback as its context. A callback cannot close over Go state, so the token
// is how it finds the scan that is waiting.
var (
	waitersMu sync.Mutex
	waiters   = map[uintptr]chan uint32{}
	nextToken atomic.Uintptr
)

// scanCallback is invoked by wlanapi on its own thread when an ACM event
// fires. It must do nothing but hand the code to the waiting scan.
var scanCallback = syscall.NewCallback(func(data *wlanNotificationData, context uintptr) uintptr {
	if data == nil || data.NotificationSource != notificationSourceACM {
		return 0
	}
	if data.NotificationCode != acmScanComplete && data.NotificationCode != acmScanFail {
		return 0
	}

	waitersMu.Lock()
	ch, ok := waiters[context]
	waitersMu.Unlock()
	if !ok {
		return 0
	}

	// Non-blocking: the scan takes the first event and stops listening, and
	// wlanapi's thread must never block on Go code.
	select {
	case ch <- data.NotificationCode:
	default:
	}
	return 0
})

// New returns the host's capture backend, failing if the host has no Wi-Fi
// interface for the WLAN service to report.
func New() (Scanner, error) {
	handle, err := openHandle()
	if err != nil {
		return nil, err
	}
	defer closeHandle(handle)

	if _, err := firstInterface(handle); err != nil {
		return nil, err
	}
	return nativeWifiScanner{}, nil
}

// Authorize has nothing to ask for: Windows gates nothing on scanning.
func Authorize() error { return nil }

// Scan implements [Scanner].
//
// The interface is resolved on every scan rather than cached at construction,
// so a USB adapter plugged in after trellisd started is picked up without a
// restart — which is how survey adapters are actually used.
func (nativeWifiScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	handle, err := openHandle()
	if err != nil {
		return nil, err
	}
	defer closeHandle(handle)

	guid, err := firstInterface(handle)
	if err != nil {
		return nil, err
	}

	if err := scanAndWait(ctx, handle, guid); err != nil {
		return nil, err
	}
	return bssList(handle, guid)
}

// openHandle opens a WLAN client handle and checks the service negotiated the
// version whose behaviour this code is written against.
func openHandle() (windows.Handle, error) {
	var negotiated uint32
	var handle windows.Handle

	ret, _, _ := procWlanOpenHandle.Call(
		uintptr(wlanClientVersion), 0,
		uintptr(unsafe.Pointer(&negotiated)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if ret != 0 {
		// ERROR_SERVICE_NOT_ACTIVE means the WLAN AutoConfig service is
		// stopped, which is a host with Wi-Fi turned off rather than one
		// without a radio, but neither can survey.
		return 0, fmt.Errorf("capture: open WLAN handle: %w", windows.Errno(ret))
	}
	return handle, nil
}

func closeHandle(handle windows.Handle) {
	_, _, _ = procWlanCloseHandle.Call(uintptr(handle), 0)
}

// firstInterface returns the GUID of the first Wi-Fi interface the WLAN service
// reports.
func firstInterface(handle windows.Handle) (windows.GUID, error) {
	// Held as unsafe.Pointer rather than uintptr throughout: wlanapi owns this
	// memory, and a uintptr that is converted back to a pointer later is a
	// misuse go vet rejects, correctly.
	var list unsafe.Pointer
	ret, _, _ := procWlanEnumInterfaces.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&list)))
	if ret != 0 {
		return windows.GUID{}, fmt.Errorf("capture: enumerate WLAN interfaces: %w", windows.Errno(ret))
	}
	defer freeMemory(list)

	count := *(*uint32)(list)
	if count == 0 {
		return windows.GUID{}, ErrNoInterface
	}

	// WLAN_INTERFACE_INFO_LIST is two DWORDs followed by the array.
	const interfacesOffset = 8
	info := (*wlanInterfaceInfo)(unsafe.Add(list, interfacesOffset))
	return info.InterfaceGUID, nil
}

// scanAndWait asks the radio to sweep and waits for the service to say it
// finished.
func scanAndWait(ctx context.Context, handle windows.Handle, guid windows.GUID) error {
	token := nextToken.Add(1)
	done := make(chan uint32, 1)

	waitersMu.Lock()
	waiters[token] = done
	waitersMu.Unlock()
	defer func() {
		waitersMu.Lock()
		delete(waiters, token)
		waitersMu.Unlock()
	}()

	// Registered before scanning: a sweep over cached channels can finish
	// quickly, and a registration made afterwards would miss the notification
	// and wait out the whole timeout.
	var previous uint32
	ret, _, _ := procWlanRegisterNotif.Call(
		uintptr(handle), uintptr(notificationSourceACM), 0,
		scanCallback, token, 0,
		uintptr(unsafe.Pointer(&previous)),
	)
	if ret != 0 {
		return fmt.Errorf("capture: subscribe to scan notifications: %w", windows.Errno(ret))
	}

	ret, _, _ = procWlanScan.Call(uintptr(handle), uintptr(unsafe.Pointer(&guid)), 0, 0, 0)
	if ret != 0 {
		return fmt.Errorf("capture: trigger scan: %w", windows.Errno(ret))
	}

	timeout := time.NewTimer(scanTimeout)
	defer timeout.Stop()

	select {
	case code := <-done:
		if code == acmScanFail {
			// The radio refused the sweep — busy, or the interface went down
			// mid-scan. Whatever it saw before that is still in the list.
			return nil
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout.C:
		return fmt.Errorf("capture: scan did not finish within %s", scanTimeout)
	}
}

// bssList reads every BSS the last scan saw.
func bssList(handle windows.Handle, guid windows.GUID) ([]wifi.ScannedNetwork, error) {
	var list unsafe.Pointer
	ret, _, _ := procWlanGetNetworkBssList.Call(
		uintptr(handle), uintptr(unsafe.Pointer(&guid)),
		0, uintptr(dot11BSSTypeAny), 0, 0,
		uintptr(unsafe.Pointer(&list)),
	)
	if ret != 0 {
		return nil, fmt.Errorf("capture: read scan results: %w", windows.Errno(ret))
	}
	defer freeMemory(list)

	// WLAN_BSS_LIST is dwTotalSize + dwNumberOfItems, then an 8-aligned array.
	const entriesOffset = 8
	count := *(*uint32)(unsafe.Add(list, 4))

	seen := time.Now().UTC()
	networks := make([]wifi.ScannedNetwork, 0, count)
	for i := range count {
		entry := (*wlanBSSEntry)(unsafe.Add(list,
			entriesOffset+uintptr(i)*unsafe.Sizeof(wlanBSSEntry{})))
		networks = append(networks, networkFromEntry(entry, seen))
	}
	return networks, nil
}

// networkFromEntry maps one WLAN_BSS_ENTRY onto Trellis's scan model.
//
// The IE blob is addressed as an offset from the entry itself, not as a
// pointer into the allocation.
func networkFromEntry(entry *wlanBSSEntry, seen time.Time) wifi.ScannedNetwork {
	var elements []element
	if entry.IeSize > 0 {
		ies := unsafe.Slice(
			(*byte)(unsafe.Add(unsafe.Pointer(entry), entry.IeOffset)),
			entry.IeSize,
		)
		elements = parseElements(ies)
	}

	// The SSID is in the entry directly as well as in the IEs. The IE is the
	// one that distinguishes a hidden network from an empty name, so it wins;
	// the struct field is the fallback for a driver that omits the element.
	ssid := ssidFromElements(elements)
	if ssid == "" && entry.SSID.Length > 0 && entry.SSID.Length <= uint32(len(entry.SSID.SSID)) {
		ssid = string(entry.SSID.SSID[:entry.SSID.Length])
	}

	freqMHz := int(entry.ChCenterFrequency / kHzPerMHz)
	channel, band := channelForFrequency(freqMHz)
	width := widthFromElements(elements)
	signal := int(entry.RSSI)

	return wifi.ScannedNetwork{
		SSID:         ssid,
		BSSID:        net.HardwareAddr(entry.BSSID[:]).String(),
		Signal:       signal,
		Channel:      channel,
		Frequency:    freqMHz,
		Security:     securityFromElements(elements, entry.CapabilityInformation),
		ChannelWidth: width,
		// Native Wifi reports no noise measurement, so the SNR below is derived
		// from the same assumed floor the other backends use when the driver
		// omits one. Consistency matters more than either being exact: a survey
		// compares points, not absolutes.
		NoiseFloor: defaultNoiseFloorDBm,
		SNR:        signal - defaultNoiseFloorDBm,
		HTMode:     htModeForWidth(width),
		IsDFS:      band == band5GHz && isDFSChannel(channel),
		LastSeen:   seen,
	}
}

func freeMemory(p unsafe.Pointer) {
	if p != nil {
		_, _, _ = procWlanFreeMemory.Call(uintptr(p))
	}
}
