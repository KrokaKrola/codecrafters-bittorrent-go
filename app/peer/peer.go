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
	Id                  []byte
	Addr                string
	HasExtensionSupport bool
	MetaDataId          int
}

const UtMetaDataId = 1
const UtPextId = 2

// Peer message ids
const (
	msgIdBitfield   = 5
	msgIdInterested = 2
	msgIdUnchoke    = 1
	msgIdExtension  = 20
)

func readMessage(conn net.Conn) (byte, []byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return 0, nil, fmt.Errorf("error reading message length from peer: %w", err)
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return 255, nil, nil
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return 0, nil, fmt.Errorf("error reading message body from peer: %w", err)
	}

	return body[0], body[1:], nil
}

func (p *Peer) sendExtensionHandshake(conn net.Conn) error {
	res, err := bencode.Encode(bencode.Dictionary{
		bencode.DictionaryValue{
			Key: "m",
			Value: bencode.Dictionary{
				bencode.DictionaryValue{
					Key:   "ut_metadata",
					Value: UtMetaDataId,
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("error trying to bencode extension msg")
	}

	extensionMsg := []byte{}
	extensionMsg = append(extensionMsg, binary.BigEndian.AppendUint32(nil, uint32(len(res)+1+1))...)
	extensionMsg = append(extensionMsg, msgIdExtension)
	extensionMsg = append(extensionMsg, 0) // extended msg id 0 = handshake
	extensionMsg = append(extensionMsg, []byte(res)...)

	if _, err := conn.Write(extensionMsg); err != nil {
		return fmt.Errorf("error writing extension handshake to peer")
	}
	return nil
}

func (p *Peer) parseExtensionHandshake(payload []byte) error {
	decoder := bencode.Decoder{Data: payload[1:]}

	decoded, ok := decoder.Decode().(bencode.Dictionary)
	if !ok {
		return fmt.Errorf("invalid response for extensions message request")
	}

	extensionDict, ok := bencode.FindElementInDictionary[bencode.Dictionary](decoded, "m")
	if !ok {
		return fmt.Errorf("invalid response for extensions message request. m dict is not found")
	}

	utMetaDataId, ok := bencode.FindElementInDictionary[int](extensionDict, "ut_metadata")
	if !ok {
		return fmt.Errorf("invalid response for extensions message request. m.ut_metadata value is not found")
	}

	p.MetaDataId = utMetaDataId
	return nil
}

func (p *Peer) sendMetadataRequest(conn net.Conn) error {
	res, err := bencode.Encode(bencode.Dictionary{
		bencode.DictionaryValue{
			Key:   "msg_type",
			Value: 0,
		},
		bencode.DictionaryValue{
			Key:   "piece",
			Value: 0,
		},
	})

	if err != nil {
		return fmt.Errorf("error trying to bencode extension metadata request")
	}

	extensionMsg := []byte{}
	extensionMsg = append(extensionMsg, binary.BigEndian.AppendUint32(nil, uint32(len(res)+1+1))...)
	extensionMsg = append(extensionMsg, msgIdExtension)
	extensionMsg = append(extensionMsg, byte(p.MetaDataId))
	extensionMsg = append(extensionMsg, []byte(res)...)

	if _, err := conn.Write(extensionMsg); err != nil {
		return fmt.Errorf("error writing extension metadata request to peer")
	}

	return nil
}

func (p *Peer) DoExtensionHandshake(conn net.Conn) error {
	if !p.HasExtensionSupport {
		return fmt.Errorf("peer does not support extensions")
	}

	if err := p.sendExtensionHandshake(conn); err != nil {
		return err
	}

	for {
		id, payload, err := readMessage(conn)
		if err != nil {
			return err
		}

		switch id {
		case msgIdExtension:
			return p.parseExtensionHandshake(payload)
		default:
			// bitfield, keep-alive, anything else: ignore and keep reading
		}
	}
}

type MetadataResponse struct {
	Length      int
	PieceLength int
	Pieces      string
}

func (p *Peer) DoMetadataRequest(conn net.Conn) (*MetadataResponse, error) {
	if !p.HasExtensionSupport {
		return nil, fmt.Errorf("peer does not support extensions")
	}

	if err := p.sendMetadataRequest(conn); err != nil {
		return nil, err
	}

	for {
		id, payload, err := readMessage(conn)
		if err != nil {
			return nil, err
		}

		switch id {
		case msgIdExtension:
			msgDecoder := bencode.Decoder{Data: payload[1:]}
			msgDecoder.Decode()

			if msgDecoder.Err != nil {
				return nil, fmt.Errorf("error trying to decode response from metadata request: %s", msgDecoder.Err.Error())
			}

			infoDecoder := bencode.Decoder{Data: payload[msgDecoder.Pos+1:]}

			res := infoDecoder.Decode()
			if infoDecoder.Err != nil {
				return nil, fmt.Errorf("error trying to decode response from metadata request: %s", infoDecoder.Err.Error())
			}

			if infoDecoder.Err != nil {
				return nil, fmt.Errorf("error trying to decode response from metadata request: %s", infoDecoder.Err.Error())
			}

			dict, ok := res.(bencode.Dictionary)
			if !ok {
				return nil, fmt.Errorf("invalid response from metadata request")
			}

			pieceLength, ok := bencode.FindElementInDictionary[int](dict, "piece length")
			if !ok {
				return nil, fmt.Errorf("piece length is not found in the metadata response")
			}

			length, ok := bencode.FindElementInDictionary[int](dict, "length")
			if !ok {
				return nil, fmt.Errorf("length is not found in the metadata response")
			}

			pieces, ok := bencode.FindElementInDictionary[string](dict, "pieces")
			if !ok {
				return nil, fmt.Errorf("pieces is not found in the metadata response")
			}

			return &MetadataResponse{
				Length:      length,
				PieceLength: pieceLength,
				Pieces:      pieces,
			}, nil

		default:
			// bitfield, keep-alive, anything else: ignore and keep reading
		}
	}
}

func (p *Peer) UnchokePeer(conn net.Conn) error {
	if p.HasExtensionSupport {
		if err := p.sendExtensionHandshake(conn); err != nil {
			return err
		}
	}

	intrestedMsg := []byte{}
	intrestedMsg = append(intrestedMsg, binary.BigEndian.AppendUint32(nil, 1)...)
	intrestedMsg = append(intrestedMsg, msgIdInterested)
	if _, err := conn.Write(intrestedMsg); err != nil {
		return fmt.Errorf("error writing interested msg to peer")
	}

	gotExtension := !p.HasExtensionSupport
	for {
		id, payload, err := readMessage(conn)
		if err != nil {
			return err
		}

		switch id {
		case msgIdExtension:
			if err := p.parseExtensionHandshake(payload); err != nil {
				return err
			}
			gotExtension = true
		case msgIdUnchoke:
			if gotExtension {
				return nil
			}
		default:
			// ignore
		}
	}
}

func (p *Peer) SendInterested(conn net.Conn) error {
	intrestedMsg := []byte{}
	intrestedMsg = append(intrestedMsg, binary.BigEndian.AppendUint32(nil, 1)...)
	intrestedMsg = append(intrestedMsg, msgIdInterested)
	if _, err := conn.Write(intrestedMsg); err != nil {
		return fmt.Errorf("error writing interested msg to peer")
	}

	for {
		id, _, err := readMessage(conn)
		if err != nil {
			return err
		}
		if id == msgIdUnchoke {
			return nil
		}
	}
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

	if res[25]&0x10 != 0 {
		peer.HasExtensionSupport = true
	}

	peer.Id = res[48:]

	return conn, nil
}
