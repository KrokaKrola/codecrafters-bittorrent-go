package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Invalid number of arguments")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "handshake":
		if len(os.Args) < 3 {
			fmt.Println("Invalid number of arguments")
			os.Exit(1)
		}
		fileName := os.Args[2]
		peerAddress := os.Args[3]

		fmt.Println(peerAddress)

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		if peerAddress == "" {
			fmt.Println("Empty peer address")
			os.Exit(1)
		}

		btFile := &bitTorrentFile{}
		err := btFile.parse(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		conn, err := net.Dial("tcp", peerAddress)
		if err != nil {
			fmt.Println("Error while trying to establish TCP connection to", peerAddress)
			os.Exit(1)
		}

		defer conn.Close()

		encodedInfo, err := encode(btFile.info)
		if err != nil {
			fmt.Printf("Error while trying to encode info content: %s", err.Error())
			os.Exit(1)
		}

		infoHash, err := createSha1HashFromString(encodedInfo, false)
		if err != nil {
			fmt.Println("Error while trying to create hash from info content")
			os.Exit(1)
		}

		message := []byte{19}
		reserved := make([]byte, 8)

		message = append(message, []byte("BitTorrent protocol")...)
		message = append(message, reserved...)
		message = append(message, []byte(infoHash)...)
		message = append(message, []byte(generateId())...)

		_, err = conn.Write(message)
		if err != nil {
			fmt.Println("Error while trying to send message to TCP connection:", err.Error())
			os.Exit(1)
		}

		res, err := io.ReadAll(conn)
		if err != nil {
			fmt.Println("Error while trying to read message from TCP connection:", err.Error())
		}

		fmt.Println("Peer ID:", hex.EncodeToString(res[48:]))
	case "peers":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		btFile := &bitTorrentFile{}
		err := btFile.parse(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		encodedInfo, err := encode(btFile.info)
		if err != nil {
			fmt.Printf("Error while trying to encode info content: %s", err.Error())
			os.Exit(1)
		}

		infoHash, err := createSha1HashFromString(encodedInfo, false)
		if err != nil {
			fmt.Println("Error while trying to create hash from info content")
			os.Exit(1)
		}

		infoLength, ok := findElementInDictionary[int](btFile.info, "length")
		if !ok {
			fmt.Println("Length field in info is not found")
			os.Exit(1)
		}

		requestUrl, err := url.Parse(btFile.announce)
		if err != nil {
			fmt.Printf("Error while parsing announce URL: %s\n", err.Error())
			os.Exit(1)
		}

		requestQuery := requestUrl.Query()
		requestQuery.Set("info_hash", string(infoHash))
		requestQuery.Set("peer_id", generateId())
		requestQuery.Set("port", "6881")
		requestQuery.Set("uploaded", "0")
		requestQuery.Set("downloaded", "0")
		requestQuery.Set("left", strconv.Itoa(infoLength))
		requestQuery.Set("compact", "1")

		requestUrl.RawQuery = requestQuery.Encode()

		client := &http.Client{}
		request, err := http.NewRequest(http.MethodGet, requestUrl.String(), nil)
		if err != nil {
			fmt.Println(err)
			fmt.Printf("Error during request to announce URL: %s\n", err.Error())
			os.Exit(1)
		}
		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("Error during request to announce URL: %s\n", err.Error())
			os.Exit(1)
		}

		body, err := io.ReadAll(response.Body)
		defer response.Body.Close()

		bodyDecoder := &decoder{data: body}
		res := bodyDecoder.decode()

		dict, ok := res.(dictionary)
		if !ok {
			fmt.Println("Invalid response from announce URL")
			os.Exit(1)
		}

		peers, ok := findElementInDictionary[string](dict, "peers")
		if !ok {
			fmt.Println("Invalid response from announce URL, peers field is not in the response")
			os.Exit(1)
		}

		peersAsBytesArr := []byte(peers)

		for i := 0; i < len(peersAsBytesArr); i += 6 {
			ip1 := peersAsBytesArr[i]
			ip2 := peersAsBytesArr[i+1]
			ip3 := peersAsBytesArr[i+2]
			ip4 := peersAsBytesArr[i+3]
			port := binary.BigEndian.Uint16(peersAsBytesArr[i+4 : i+6])

			fmt.Printf("%d.%d.%d.%d:%d\n", ip1, ip2, ip3, ip4, port)
		}
	case "info":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		btFile := &bitTorrentFile{}
		err := btFile.parse(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var encodedPieces []string

		pieces, ok := findElementInDictionary[string](btFile.info, "pieces")
		if !ok {
			fmt.Println("Pieces field is not found")
			os.Exit(1)
		}

		for i := 0; i < len(pieces); i += 20 {
			encodedPieces = append(encodedPieces, hex.EncodeToString([]byte(pieces[i:i+20])))
		}

		encodedInfo, err := encode(btFile.info)
		if err != nil {
			fmt.Printf("Error while trying to encode info content: %s", err.Error())
			os.Exit(1)
		}

		hash, err := createSha1HashFromString(encodedInfo, true)
		if err != nil {
			fmt.Println("Error while trying to create hash from info content")
			os.Exit(1)
		}

		infoLength, ok := findElementInDictionary[int](btFile.info, "length")
		if !ok {
			fmt.Println("Length field in info is not found")
			os.Exit(1)
		}

		pieceLength, ok := findElementInDictionary[int](btFile.info, "piece length")
		if !ok {
			fmt.Println("Piece length filed in info is not found")
			os.Exit(1)
		}

		fmt.Println("Tracker URL:", btFile.announce)
		fmt.Println("Length:", infoLength)
		fmt.Println("Info Hash:", hash)
		fmt.Println("Piece Length:", pieceLength)
		fmt.Println("Piece Hashes:")
		for _, el := range encodedPieces {
			fmt.Println(el)
		}
	case "decode":
		bencodedValue := os.Args[2]

		dec := decoder{
			data: []byte(bencodedValue),
		}

		decoded := dec.decode()

		if dec.err != nil {
			fmt.Println(dec.err)
			return
		}

		if _, ok := decoded.(dictionary); ok {
			decoded = decoded.(dictionary).toMap()
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
