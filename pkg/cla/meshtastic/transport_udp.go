package meshtastic

import "net"

// UDPTransport implements Transport using localhost UDP datagrams.
// This is the simulation mode used to test chunking without hardware.
type UDPTransport struct {
    sendConn *net.UDPConn
    recvConn *net.UDPConn
}

func NewUDPTransport(listenAddr, peerAddr string) (*UDPTransport, error) {
    lAddr, err := net.ResolveUDPAddr("udp", listenAddr)
    if err != nil {
        return nil, err
    }
    recvConn, err := net.ListenUDP("udp", lAddr)
    if err != nil {
        return nil, err
    }

    pAddr, err := net.ResolveUDPAddr("udp", peerAddr)
    if err != nil {
        recvConn.Close()
        return nil, err
    }
    sendConn, err := net.DialUDP("udp", nil, pAddr)
    if err != nil {
        recvConn.Close()
        return nil, err
    }

    return &UDPTransport{sendConn: sendConn, recvConn: recvConn}, nil
}

func (t *UDPTransport) SendPacket(data []byte) error {
    _, err := t.sendConn.Write(data)
    return err
}

func (t *UDPTransport) ReceivePacket() ([]byte, error) {
    buf := make([]byte, 200)
    n, _, err := t.recvConn.ReadFromUDP(buf)
    return buf[:n], err
}

func (t *UDPTransport) Close() error {
    t.sendConn.Close()
    return t.recvConn.Close()
}