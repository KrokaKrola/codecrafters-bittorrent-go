package torrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type Peer struct {
	Id   []byte
	Conn net.Conn
	Addr string
}

func (p *Peer) UnchokePeer() error {
	bitfieldMsgSizeBuff := make([]byte, 4)
	if _, err := io.ReadFull(p.Conn, bitfieldMsgSizeBuff); err != nil {
		return fmt.Errorf("error reading bitfield msg size from peer")
	}

	bitfieldMsg := make([]byte, binary.BigEndian.Uint32(bitfieldMsgSizeBuff))
	if _, err := io.ReadFull(p.Conn, bitfieldMsg); err != nil {
		return fmt.Errorf("error reading bitfield msg from peer")
	}

	intrestedMsg := []byte{}
	intrestedMsg = append(intrestedMsg, binary.BigEndian.AppendUint32(nil, 1)...)
	intrestedMsg = append(intrestedMsg, []byte{2}...)

	if _, err := p.Conn.Write(intrestedMsg); err != nil {
		return fmt.Errorf("error writing intrested msg to peer")
	}

	unchokeMsgSizeBuff := make([]byte, 4)
	if _, err := io.ReadFull(p.Conn, unchokeMsgSizeBuff); err != nil {
		return fmt.Errorf("error reading unchoke msg size from peer")
	}

	unchokeMsg := make([]byte, binary.BigEndian.Uint32(unchokeMsgSizeBuff))
	if _, err := io.ReadFull(p.Conn, unchokeMsg); err != nil {
		return fmt.Errorf("error reading unchoke msg from peer")
	}

	return nil
}
