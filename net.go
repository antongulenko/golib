package golib

import (
	"errors"
	"net"
	"sync"
)

// FirstIpAddress tries to get the main public IP of the local host.
// It iterates all available, enabled network interfaces and looks for the first
// non-local IP address.
func FirstIpAddress() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			// Loopback and disabled interfaces are not interesting
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.IP, nil
			case *net.IPAddr:
				return v.IP, nil
			}
		}
	}
	return nil, errors.New("No valid network interfaces found")
}

// ==================== TCP listener task ====================

// TCPConnectionHandler is a callback function for TCPListenerTask, which is
// invoked whenever a new TCP connection is successfully accepted.
type TCPConnectionHandler func(wg *sync.WaitGroup, conn *net.TCPConn)

// TCPListenerTask is an implementation of the Task interface that listens
// for incoming TCP connections on a given TCP endpoint. A handler function
// is invoked for every accepted TCP connection, and an optional hook can be
// executed when the TCP socket is closed and the task stops.
type TCPListenerTask struct {
	*LoopTask

	// ListenEndpoint is the TCP endpoint to open a TCP listening socket on.
	ListenEndpoint string

	// Handler is a required callback-function that will be called for every
	// successfully established TCP connection. It is not called in a separate
	// goroutine, so it should fork a new routine for long-running connections.
	// The handler is always executed while the StopChan in the underlying
	// LoopTask is locked.
	Handler TCPConnectionHandler

	// StopHook is an optional callback that is invoked after the task stops and
	// the listening TCP socket is closed. When StopHook is executed, the underlying
	// LoopTask/StopChan is NOT locked, so helpers methods like Execute() must be used
	// if synchronization is required.
	StopHook func()

	listener *net.TCPListener
}

// String implements the Task interface by returning a descriptive string.
func (task *TCPListenerTask) String() string {
	return "TCP listener " + task.ListenEndpoint
}

// Start implements the Task interface. It opens the TCP listen socket and
// starts accepting incoming connections.
func (task *TCPListenerTask) Start(wg *sync.WaitGroup) StopChan {
	return task.ExtendedStart(nil, wg)
}

// ExtendedStart creates the TCP listen socket and starts accepting incoming connections.
// In addition, a hook function can be defined that will be called once after the
// socket has been opened successfully and is passed the resolved address of the TCP endpoint.
func (task *TCPListenerTask) ExtendedStart(start func(addr net.Addr), wg *sync.WaitGroup) StopChan {
	hook := task.StopHook
	defer func() {
		if hook != nil {
			hook()
		}
	}()
	task.LoopTask = task.listen(wg)

	endpoint, err := net.ResolveTCPAddr("tcp", task.ListenEndpoint)
	if err != nil {
		return NewStoppedChan(err)
	}
	task.listener, err = net.ListenTCP("tcp", endpoint)
	if err != nil {
		return NewStoppedChan(err)
	}
	if start != nil {
		start(task.listener.Addr())
	}
	hook = nil
	return task.LoopTask.Start(wg)
}

func (task *TCPListenerTask) listen(wg *sync.WaitGroup) *LoopTask {
	return &LoopTask{
		Description: "tcp listener on " + task.ListenEndpoint,
		StopHook:    task.StopHook,
		Loop: func(stop StopChan) error {
			if listener := task.listener; listener == nil {
				return StopLoopTask
			} else {
				conn, err := listener.AcceptTCP()
				if err != nil {
					if task.listener != nil {
						Log.Errorln("Error accepting connection:", err)
					}
				} else {
					stop.IfElseStopped(func() {
						_ = conn.Close() // Drop error
					}, func() {
						task.Handler(wg, conn)
					})
				}
			}
			return nil
		},
	}
}

// StopErrFunc extends the StopErrFunc() function inherited from LoopTask/StopChan and additionally
// closes the TCP listening socket.
func (task *TCPListenerTask) StopErrFunc(perform func() error) {
	task.LoopTask.StopErrFunc(func() error {
		task.stop()
		return perform()
	})
}

// StopFunc extends the StopFunc() function inherited from LoopTask/StopChan and additionally
// closes the TCP listening socket.
func (task *TCPListenerTask) StopFunc(perform func()) {
	task.LoopTask.StopFunc(func() {
		task.stop()
		perform()
	})
}

// StopErr extends the StopErr() function inherited from LoopTask/StopChan and additionally
// closes the TCP listening socket.
func (task *TCPListenerTask) StopErr(err error) {
	task.LoopTask.StopErrFunc(func() error {
		task.stop()
		return err
	})
}

// Stop extends the Stop() function inherited from LoopTask/StopChan and additionally
// closes the TCP listening socket.
func (task *TCPListenerTask) Stop() {
	task.LoopTask.StopFunc(func() {
		task.stop()
	})
}

func (task *TCPListenerTask) stop() {
	if listener := task.listener; listener != nil {
		task.listener = nil  // Will be checked when returning from AcceptTCP()
		_ = listener.Close() // Drop error
	}
}

// ==================== UDP listener task ====================

var DefaultUdpPacketSize = 2048

// UDPConnectionHandler is a callback function for UDPListenerTask, which is
// invoked whenever a new UDP packet is received.
type UDPPacketHandler func(wg *sync.WaitGroup, localAddr net.Addr, remoteAddr *net.UDPAddr, packet []byte)

// UDPListenerTask is an implementation of the Task interface that listens
// for incoming UDP packets on a given UDP endpoint. A handler function
// is invoked for every received UDP packet, and an optional hook can be
// executed when the UDP socket is closed and the task stops.
type UDPListenerTask struct {
	*LoopTask

	// ListenEndpoint is the UDP endpoint to open a UDP listening socket on.
	ListenEndpoint string

	// Handler is a required callback-function that will be called for every
	// received UDP packet. It is not called in a separate
	// goroutine, so it should fork a new routine for long-running operations.
	// The handler is always executed while the StopChan in the underlying
	// LoopTask is locked.
	Handler UDPPacketHandler

	// StopHook is an optional callback that is invoked after the task stops and
	// the listening UDP socket is closed. When StopHook is executed, the underlying
	// LoopTask/StopChan is NOT locked, so helper methods like Execute() must be used
	// if synchronization is required.
	StopHook func()

	// The size of the buffer to receive UDP packets into. If the buffer size is smaller
	// than incoming packets, the UDPPacketHandler might be invoked multiple times with
	// different UDP packet fragments. The value of DefaultUdpPacketSize is used if this is
	// to <=0.
	PacketBufferSize int

	listener *net.UDPConn
}

// String implements the Task interface by returning a descriptive string.
func (task *UDPListenerTask) String() string {
	return "UDP listener " + task.ListenEndpoint
}

// Start implements the Task interface. It opens the UDP listen socket and
// starts accepting incoming packets.
func (task *UDPListenerTask) Start(wg *sync.WaitGroup) StopChan {
	return task.ExtendedStart(nil, wg)
}

// ExtendedStart creates the UDP listen socket and starts receiving incoming packets.
// In addition, a hook function can be defined that will be called once after the
// socket has been opened successfully and is passed the resolved address of the UDP endpoint.
func (task *UDPListenerTask) ExtendedStart(start func(addr net.Addr), wg *sync.WaitGroup) StopChan {
	hook := task.StopHook
	defer func() {
		if hook != nil {
			hook()
		}
	}()
	task.LoopTask = task.listen(wg)

	endpoint, err := net.ResolveUDPAddr("udp", task.ListenEndpoint)
	if err != nil {
		return NewStoppedChan(err)
	}
	task.listener, err = net.ListenUDP("udp", endpoint)
	if err != nil {
		return NewStoppedChan(err)
	}
	if start != nil {
		start(task.listener.LocalAddr())
	}
	hook = nil
	return task.LoopTask.Start(wg)
}

func (task *UDPListenerTask) listen(wg *sync.WaitGroup) *LoopTask {
	return &LoopTask{
		Description: "udp listener on " + task.ListenEndpoint,
		StopHook:    task.StopHook,
		Loop: func(stop StopChan) error {
			if listener := task.listener; listener == nil {
				return StopLoopTask
			} else {
				// TODO recycle these buffers for performance
				bufLen := task.PacketBufferSize
				if bufLen <= 0 {
					bufLen = DefaultUdpPacketSize
				}
				buf := make([]byte, bufLen)
				num, remoteAddr, err := listener.ReadFromUDP(buf)
				buf = buf[:num]
				if err != nil {
					if task.listener != nil {
						Log.Errorln("Error accepting UDP packet:", err)
					}
				} else {
					stop.IfNotStopped(func() {
						task.Handler(wg, listener.LocalAddr(), remoteAddr, buf)
					})
				}
			}
			return nil
		},
	}
}

// StopErrFunc extends the StopErrFunc() function inherited from LoopTask/StopChan and additionally
// closes the UDP listening socket.
func (task *UDPListenerTask) StopErrFunc(perform func() error) {
	task.LoopTask.StopErrFunc(func() error {
		task.stop()
		return perform()
	})
}

// StopFunc extends the StopFunc() function inherited from LoopTask/StopChan and additionally
// closes the UDP listening socket.
func (task *UDPListenerTask) StopFunc(perform func()) {
	task.LoopTask.StopFunc(func() {
		task.stop()
		perform()
	})
}

// StopErr extends the StopErr() function inherited from LoopTask/StopChan and additionally
// closes the UDP listening socket.
func (task *UDPListenerTask) StopErr(err error) {
	task.LoopTask.StopErrFunc(func() error {
		task.stop()
		return err
	})
}

// Stop extends the Stop() function inherited from LoopTask/StopChan and additionally
// closes the UDP listening socket.
func (task *UDPListenerTask) Stop() {
	task.LoopTask.StopFunc(func() {
		task.stop()
	})
}

func (task *UDPListenerTask) stop() {
	if listener := task.listener; listener != nil {
		task.listener = nil  // Will be checked when returning from AcceptTCP()
		_ = listener.Close() // Drop error
	}
}
