package listener

import (
	"github.com/galaco/bitbuf"
	"github.com/Equoo/sourcenet"
	"github.com/Equoo/sourcenet/message"
	"github.com/Equoo/update-steamworks"
	"github.com/Equoo/update-steamworks/steamauth"
	"log"
)

const (
	netSignOnState = 6
)

// Connector is a standard mechanism for connecting to source engine servers
// Many games implement the same communication, particularly early games. It
// will handle connectionless back-and-forth with a server, until we get
// a successfully connected message back from the server.
type Connector struct {
	playerName  string
	password    string
	gameVersion string

	clientChallenge int32
	serverChallenge int32

	activeClient   *sourcenet.Client
	connectionStep int32
}

// Register provides a mechanism for a listener to respond
// back to the client. This allows for encapsulation of certain
// back-and-forth logic for authentication.
func (listener *Connector) Register(client *sourcenet.Client) {
	listener.activeClient = client
}

// Receive is a callback that receives a message that the client
// received from the connected server.
func (listener *Connector) Receive(msg sourcenet.IMessage, msgType int) {
	if msg.Connectionless() == false {
		listener.handleConnected(msg, msgType)
	}

	listener.handleConnectionless(msg)
}

// InitialMessage Get the first message to initialize
// server authentication before connection.
func (listener *Connector) InitialMessage() sourcenet.IMessage {
	return message.ConnectionlessQ(listener.clientChallenge)
}

// handleConnectionless: Connectionless messages handler
func (listener *Connector) handleConnectionless(msg sourcenet.IMessage) {
	packet := bitbuf.NewReader(msg.Data())

	_,_ = packet.ReadInt32() // connectionless header

	packetType, _ := packet.ReadUint8()

	switch packetType {
	// 'A' is connection request acknowledgement.
	// We are required to authenticate game ownership now.
	case 'A':
		listener.connectionStep = 2
		_,_ = packet.ReadInt32()
		serverChallenge, _ := packet.ReadInt32()
		clientChallenge, _ := packet.ReadInt32()

		listener.serverChallenge = serverChallenge
		listener.clientChallenge = clientChallenge

		steamId64 := uint64(steamworks.GetSteamID())
		steamKey, _ := steamauth.CreateTicket()

		msg := message.ConnectionlessK(
			listener.clientChallenge,
			listener.serverChallenge,
			listener.playerName,
			listener.password,
			listener.gameVersion,
			steamId64,
			steamKey)

		listener.activeClient.SendMessage(msg, false)
	// 'B' is successful authentication.
	// Now send some user info bits.
	case 'B':
		if listener.connectionStep == 2 {
			log.Println("Connected successfully")
			listener.connectionStep = 3

			sendData := bitbuf.NewWriter(2048)

			sendData.WriteUnsignedBitInt32(6, 6)
			sendData.WriteByte(2)
			sendData.WriteInt32(-1)

			sendData.WriteUnsignedBitInt32(4, 8)
			sendData.WriteBytes([]byte("VModEnable 1"))
			sendData.WriteByte(0)
			sendData.WriteUnsignedBitInt32(4, 6)
			sendData.WriteString("vban 0 0 0 0")
			sendData.WriteByte(0)

			listener.activeClient.SendMessage(message.NewGeneric(sendData.Data()), false)
		}
	// '9' Connection was refused. A reason
	// is usually provided.
	case '9':
		_,_ = packet.ReadInt32() // Not needed
		reason, _ := packet.ReadString(1024)
		log.Printf("Connection refused. Reason: %s\n", reason)
	default:
		return
	}
}

// handleConnected Connected message handler
func (listener *Connector) handleConnected(msg sourcenet.IMessage, msgType int) {
	if msgType != netSignOnState {
		return
	}
}

// NewConnector returns a new connector object.
func NewConnector(playerName string, password string, gameVersion string, clientChallenge int32) *Connector {
	return &Connector{
		playerName:      playerName,
		password:        password,
		gameVersion:     gameVersion,
		clientChallenge: clientChallenge,
		connectionStep:  1,
	}
}
