//go:build windows

// BRB02 图传探针: 在本机复现工具箱验证过的上传序列, 逐帧计时并收集设备应答。
// 画布 = 纯红 428x142 (成功则屏幕变红, 一眼确认)。
package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"golang.org/x/sys/windows"
)

const (
	vid, pid            = 0xE2B7, 0x7001
	epOut, epIn         = 0x01, 0x81
	wireLen             = 64
	baseMS              = 7.0
	pagePauseMS         = 40.0
	ackTimeout          = 3 * time.Second
	firstBlockAfterACK  = 4200 * time.Microsecond // 工具箱: ACK -> 首块 4.2ms
	minWriteGap         = 5 * time.Millisecond
	pageSize            = 4096
	blockPayload        = 58
)

var (
	dll                 *windows.LazyDLL
	procInit            *windows.LazyProc
	procExit            *windows.LazyProc
	procOpen            *windows.LazyProc
	procClose           *windows.LazyProc
	procClaim           *windows.LazyProc
	procRelease         *windows.LazyProc
	procInterrupt       *windows.LazyProc
	procAutoDetach      *windows.LazyProc
)

func loadDLL() {
	candidates := []string{
		`D:\FanControl\libusb-1.0.dll`,
		`C:\Windows\System32\libusb-1.0.dll`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		dll = windows.NewLazyDLL(c)
		if err := dll.Load(); err != nil {
			fmt.Println("load fail:", c, err)
			continue
		}
		procInit = dll.NewProc("libusb_init")
		procExit = dll.NewProc("libusb_exit")
		procOpen = dll.NewProc("libusb_open_device_with_vid_pid")
		procClose = dll.NewProc("libusb_close")
		procClaim = dll.NewProc("libusb_claim_interface")
		procRelease = dll.NewProc("libusb_release_interface")
		procInterrupt = dll.NewProc("libusb_interrupt_transfer")
		procAutoDetach = dll.NewProc("libusb_set_auto_detach_kernel_driver")
		fmt.Println("DLL:", c)
		return
	}
	panic("libusb-1.0.dll not found")
}

type device struct{ ctx, handle uintptr }

func openDevice() *device {
	var ctx uintptr
	if r, _, _ := procInit.Call(uintptr(unsafe.Pointer(&ctx))); int32(r) != 0 {
		panic("init failed")
	}
	h, _, _ := procOpen.Call(ctx, vid, pid)
	if h == 0 {
		procExit.Call(ctx)
		panic(fmt.Sprintf("device not found (VID %04X PID %04X)", vid, pid))
	}
	procAutoDetach.Call(h, 1)
	procSetConfig := dll.NewProc("libusb_set_configuration")
	procSetConfig.Call(h, 1)
	if r, _, _ := procClaim.Call(h, 0); int32(r) != 0 {
		procClose.Call(h)
		procExit.Call(ctx)
		panic(fmt.Sprintf("claim failed: %d", int32(r)))
	}
	return &device{ctx: ctx, handle: h}
}

func (d *device) close() {
	procRelease.Call(d.handle, 0)
	procClose.Call(d.handle)
	procExit.Call(d.ctx)
}

// transfer OUT: 发送 wire 字节; IN: 读取至多 len(buf) 字节。返回 (transferred, err)。
func (d *device) transfer(endpoint byte, buf []byte, timeoutMS uint32) (int, error) {
	var transferred int32
	r, _, _ := procInterrupt.Call(
		d.handle, uintptr(endpoint), uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)), uintptr(unsafe.Pointer(&transferred)), uintptr(timeoutMS),
	)
	return int(transferred), errnoErr(int32(r), endpoint)
}

func errnoErr(r int32, endpoint byte) error {
	if r == 0 {
		return nil
	}
	return fmt.Errorf("endpoint 0x%02X rc=%d", endpoint, r)
}

type rx struct {
	at     time.Time
	cmd    byte
	status byte
	raw    []byte
}

func main() {
	loadDLL()
	dev := openDevice()
	defer dev.close()

	// 纯红画布
	canvas := make([]byte, deviceproto.BlackSharkImageBytes)
	for i := 0; i < len(canvas); i += 2 {
		canvas[i], canvas[i+1] = 0xF8, 0x00
	}
	frames, err := deviceproto.BuildBlackSharkImageUpload(canvas)
	if err != nil {
		panic(err)
	}
	fmt.Printf("frames: %d (C4 + 2095 blocks + tail + C5)\n", len(frames))

	// 心跳帧 (0x07, 全 0 项): 与上传序列一致的 26 字节
	hb := []byte{0xA5, 0x1A, 0x07, 0x07}
	for i := 0; i < 7; i++ {
		hb = append(hb, byte(i), 0x00, 0x00)
	}
	sum := byte(0)
	for _, b := range hb {
		sum += b
	}
	hb = append(hb, sum)

	// 读线程: 20ms 短超时轮询 IN, 记录 (时刻, cmd, status)
	acks := make(chan rx, 4096)
	stop := make(chan struct{})
	go func() {
		// 设备中断端点 wMaxPacketSize=65 (违规但实测如此), IN 包固定 65 字节
		buf := make([]byte, 65)
		for {
			select {
			case <-stop:
				return
			default:
			}
			n, err := dev.transfer(epIn, buf, 20)
			if err != nil || n < 5 {
				continue
			}
			pkt := append([]byte(nil), buf[:n]...)
			if pkt[0] == 0xA5 && pkt[1] == 0x05 {
				acks <- rx{at: time.Now(), cmd: pkt[2], status: pkt[3], raw: pkt}
			}
		}
	}()
	drain := func() {
		for {
			select {
			case <-acks:
			default:
				return
			}
		}
	}

	// 1) C4 握手
	drain()
	t0 := time.Now()
	if _, err := dev.transfer(epOut, pad64(frames[0]), 800); err != nil {
		panic(err)
	}
	fmt.Println("C4 sent at t0")

	var ackAt time.Time
	ackCmd := byte(0)
	deadline := time.Now().Add(ackTimeout)
	for time.Now().Before(deadline) {
		select {
		case r := <-acks:
			lat := r.at.Sub(t0)
			fmt.Printf("  ACK candidate: cmd=0x%02X status=0x%02X latency=%v\n", r.cmd, r.status, lat.Round(time.Microsecond))
			if r.status == 0 && ackAt.IsZero() {
				ackAt, ackCmd = r.at, r.cmd
			}
		default:
			time.Sleep(2 * time.Millisecond)
		}
		if !ackAt.IsZero() {
			break
		}
	}
	if ackAt.IsZero() {
		close(stop)
		panic("C4 无 ACK (3s) —— 与 20:04 的现象一致")
	}
	fmt.Printf("handshake OK: cmd=0x%02X ackLatency=%v\n", ackCmd, ackAt.Sub(t0).Round(time.Microsecond))

	// 2) 首块锚定 ACK+4.2ms
	if d := ackAt.Add(firstBlockAfterACK).Sub(time.Now()); d > 0 {
		time.Sleep(d)
	}

	// 3) 数据块流: 7ms 基速 + 页停顿, 防突发 >=5ms
	var firstBad int = -1
	badCount, goodCount := 0, 0
	statusHist := map[byte]int{}
	lastWrite := time.Now()
	blocks := frames[1 : len(frames)-1]
	tBegin := time.Now()
	for i, fr := range blocks {
		if d := lastWrite.Add(minWriteGap).Sub(time.Now()); d > 0 {
			time.Sleep(d)
		}
		start := i * blockPayload
		if i > 0 && start/pageSize != (start-blockPayload)/pageSize {
			time.Sleep(time.Duration(pagePauseMS) * time.Millisecond)
		}
		if _, err := dev.transfer(epOut, pad64(fr), 800); err != nil {
			fmt.Printf("block %d write error: %v\n", i+1, err)
			return
		}
		lastWrite = time.Now()
		// 收割已到达的 C6
		for {
			select {
			case r := <-acks:
				statusHist[r.status]++
				if r.cmd == 0xC6 || r.cmd == 0xC4 {
					if r.status == 0 {
						goodCount++
					} else {
						badCount++
						if firstBad < 0 {
							firstBad = i + 1
							fmt.Printf("  FIRST REJECT after block %d: cmd=0x%02X status=0x%02X at +%.1fms\n",
								i+1, r.cmd, r.status, float64(r.at.Sub(tBegin).Microseconds())/1000.0)
						}
					}
				}
				continue
			default:
			}
			break
		}
		if badCount >= 5 {
			fmt.Println("  0x0C 级联, 提前中止")
			break
		}
	}
	elapsed := time.Since(tBegin)
	fmt.Printf("blocks done: good=%d bad=%d firstBad=%d elapsed=%v\n", goodCount, badCount, firstBad, elapsed.Round(time.Millisecond))
	fmt.Printf("status hist: %v\n", statusHist)

	if badCount > 0 {
		close(stop)
		fmt.Println("RESULT: 设备拒绝了数据块 —— 需要继续定位")
		return
	}

	// 4) 尾部: 64ms -> 16 心跳 -> C5
	time.Sleep(64 * time.Millisecond)
	for i := 0; i < 16; i++ {
		dev.transfer(epOut, pad64(hb), 800)
		time.Sleep(6 * time.Millisecond)
	}
	time.Sleep(6 * time.Millisecond)
	if _, err := dev.transfer(epOut, pad64(frames[len(frames)-1]), 800); err != nil {
		fmt.Println("C5 write error:", err)
		return
	}
	time.Sleep(500 * time.Millisecond)
	drain()
	fmt.Println("RESULT: 全流程发送完成 (C6 全 0x00 含尾帧) —— 屏幕应为纯红")
	close(stop)
}

func pad64(frame []byte) []byte {
	out := make([]byte, wireLen)
	copy(out, frame)
	return out
}
