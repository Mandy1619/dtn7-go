package meshtastic

import "net"

// UDPTransport implements Transport using localhost UDP datagrams. Why UDP for simulation? -  LoRa packets have same one packet = one datagramconcept. 
// This is the simulation mode used to test chunking without hardware.

// Usage in node.toml (simulation mode):
//   [[Listener]]
//   type    = "meshtastic"
//   address = "0.0.0.0:5005"   ->  we listen on this port
//   peer    = "127.0.0.1:5006" -> we send TO this port (node2's address)
//   peer_id = "dtn://node2/"

type UDPTransport struct {
    sendConn *net.UDPConn //connection we write chunks TO (the perr's port)
    recvConn *net.UDPConn // socket we read chunks FROM (our own port)
}

//NewUDPTransport opens both a listen sock (recvConn) and a send Connection (sendConn)
func NewUDPTransport(listenAddr, peerAddr string) (*UDPTransport, error) {
	//open our recieve socket
    lAddr, err := net.ResolveUDPAddr("udp", listenAddr)
    if err != nil {
        return nil, err
    }
    recvConn, err := net.ListenUDP("udp", lAddr)
    if err != nil {
        return nil, err
    }

	//OPen our send Connection to the peer
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

//SendPacket writes one UDP Datagram to the peer
func (t *UDPTransport) SendPacket(data []byte) error {
    _, err := t.sendConn.Write(data)
    return err
}

//RecievePacket blocks until a UDP Datagram arrives, then returns its bytes
func (t *UDPTransport) ReceivePacket() ([]byte, error) {
    buf := make([]byte, 200) //Buffer: 200
    n, _, err := t.recvConn.ReadFromUDP(buf)
    return buf[:n], err
}

func (t *UDPTransport) Close() error {
    t.sendConn.Close()
    return t.recvConn.Close()
}
