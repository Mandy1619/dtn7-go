package meshtastic

import "net"

// UDPTransport implements Transport using localhost UDP datagrams.
// This is the simulation mode used to test
type UDPTransport struct {
    sendConn *net.UDPConn    // the conn currently in MeshtasticClient
    recvConn *net.UDPConn    // the conn currently in MeshtasticServer
}

func NewUDPTransport(listenAddr, peerAddr string) (*UDPTransport, error) {
    // move the net.ResolveUDPAddr + net.ListenUDP + net.DialUDP
    // calls that currently live in Activate() and Start() — put them here
}

func (t *UDPTransport) SendPacket(data []byte) error {
    _, err := t.sendConn.Write(data)   // exact same line as before
    return err
}

func (t *UDPTransport) ReceivePacket() ([]byte, error) {
    buf := make([]byte, 200)
    n, _, err := t.recvConn.ReadFromUDP(buf)   // exact same call as before
    return buf[:n], err
}

func (t *UDPTransport) Close() error {
    t.sendConn.Close()
    return t.recvConn.Close()
}