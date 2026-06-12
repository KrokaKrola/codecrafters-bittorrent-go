package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/app/utils/stringutil"
)

type Peer struct {
	Id   []byte
	Addr string
}

func (p *Peer) UnchokePeer(conn net.Conn) error {
	bitfieldMsgSizeBuff := make([]byte, 4)
	if _, err := io.ReadFull(conn, bitfieldMsgSizeBuff); err != nil {
		return fmt.Errorf("error reading bitfield msg size from peer")
	}

	bitfieldMsg := make([]byte, binary.BigEndian.Uint32(bitfieldMsgSizeBuff))
	if _, err := io.ReadFull(conn, bitfieldMsg); err != nil {
		return fmt.Errorf("error reading bitfield msg from peer")
	}

	intrestedMsg := []byte{}
	intrestedMsg = append(intrestedMsg, binary.BigEndian.AppendUint32(nil, 1)...)
	intrestedMsg = append(intrestedMsg, []byte{2}...)

	if _, err := conn.Write(intrestedMsg); err != nil {
		return fmt.Errorf("error writing intrested msg to peer")
	}

	unchokeMsgSizeBuff := make([]byte, 4)
	if _, err := io.ReadFull(conn, unchokeMsgSizeBuff); err != nil {
		return fmt.Errorf("error reading unchoke msg size from peer")
	}

	unchokeMsg := make([]byte, binary.BigEndian.Uint32(unchokeMsgSizeBuff))
	if _, err := io.ReadFull(conn, unchokeMsg); err != nil {
		return fmt.Errorf("error reading unchoke msg from peer")
	}

	return nil
}

func GetPeers(trackerUrl string, hash string, left int) ([]*Peer, error) {
	requestUrl, err := url.Parse(trackerUrl)
	if err != nil {
		return nil, fmt.Errorf("Error while parsing announce URL: %s\n", err.Error())
	}

	requestQuery := requestUrl.Query()
	requestQuery.Set("info_hash", hash)
	requestQuery.Set("peer_id", stringutil.GenerateId())
	requestQuery.Set("port", "6881")
	requestQuery.Set("uploaded", "0")
	requestQuery.Set("downloaded", "0")
	requestQuery.Set("left", strconv.Itoa(left))
	requestQuery.Set("compact", "1")

	requestUrl.RawQuery = requestQuery.Encode()

	client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error during request to announce URL: %s\n", err.Error())
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Error during request to announce URL: %s\n", err.Error())
	}

	body, err := io.ReadAll(response.Body)
	defer response.Body.Close()

	bodyDecoder := &bencode.Decoder{Data: body}
	res := bodyDecoder.Decode()

	dict, ok := res.(bencode.Dictionary)
	if !ok {
		return nil, fmt.Errorf("Invalid response from announce URL")
	}

	peers, ok := bencode.FindElementInDictionary[string](dict, "peers")
	if !ok {
		return nil, fmt.Errorf("Invalid response from announce URL, peers field is not in the response")
	}

	peersAsBytesArr := []byte(peers)
	var result []*Peer

	for i := 0; i < len(peersAsBytesArr); i += 6 {
		ipPartOne := peersAsBytesArr[i]
		ipPartTwo := peersAsBytesArr[i+1]
		ipPartThree := peersAsBytesArr[i+2]
		ipPartFour := peersAsBytesArr[i+3]
		port := binary.BigEndian.Uint16(peersAsBytesArr[i+4 : i+6])

		result = append(result, &Peer{
			Addr: fmt.Sprintf("%d.%d.%d.%d:%d", ipPartOne, ipPartTwo, ipPartThree, ipPartFour, port),
		})
	}

	return result, nil
}

func (peer *Peer) HandshakeWithPeer(infoHash string, extension bool) (net.Conn, error) {
	conn, err := net.Dial("tcp", peer.Addr)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to establish TCP connection to: %s. Error: %s", peer.Addr, err.Error())
	}

	message := []byte{19}
	reserved := make([]byte, 8)

	if extension {
		reserved[5] = 0x10
	}

	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, reserved...)
	message = append(message, []byte(infoHash)...)
	message = append(message, []byte(stringutil.GenerateId())...)

	if _, err = conn.Write(message); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error while trying to send message to TCP connection: %s", err.Error())
	}

	res := make([]byte, 68)
	if _, err := io.ReadFull(conn, res); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error while trying to read message from TCP connection: %s", err.Error())
	}

	peer.Id = res[48:]

	return conn, nil
}
