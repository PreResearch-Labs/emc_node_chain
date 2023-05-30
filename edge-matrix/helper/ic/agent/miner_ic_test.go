package agent

import (
	"github.com/emc-protocol/edge-matrix/helper/hex"
	"github.com/emc-protocol/edge-matrix/helper/ic/utils/identity"
	"github.com/emc-protocol/edge-matrix/helper/ic/utils/principal"
	"math/big"
	"testing"

	"github.com/emc-protocol/edge-matrix/helper/ic/utils/idl"
)

const (
	privKeyHexString = "833fe62409237b9d62ec77587520911e9a759cec1d19755b7da901b96dca3d42"
	NodeRouter       = 0
	NodeValidator    = 1
	NodeComputing    = 2
)

func TestHello_QueryRaw(t *testing.T) {
	//EXT canister
	//canisterID := "bzsui-sqaaa-aaaah-qce2a-cai"

	//PUNK canister
	// canisterID := "qfh5c-6aaaa-aaaah-qakeq-cai"

	//agent := New(true, "")
	agent := New(false, "833fe62409237b9d62ec77587520911e9a759cec1d19755b7da901b96dca3d42")

	canister := "xb3xh-uaaaa-aaaam-abi3a-cai"
	methodName := "greet"

	var argType []idl.Type
	argType = append(argType, new(idl.Text))

	var argValue []interface{}
	argValue = append(argValue, "world")

	arg, _ := idl.Encode(argType, argValue)
	_, result, str, err := agent.Query(canister, methodName, arg)
	if err != nil {
		panic(err)
	}
	t.Log(str, " ->", result[0])
}

func Test_local_whoAmI(t *testing.T) {
	// generate ed25519 key for ICP identity
	//privKeyBytes, ed25519PrivKeyStringBytes, _ := crypto.GenerateAndEncodeICPIdentitySecretKey()
	//t.Log("privKeyBytesHexString: ", hex.EncodeToString(privKeyBytes))
	//t.Log("ed25519PrivKeyString: ", string(ed25519PrivKeyStringBytes))
	//decodedPrivKey, err := crypto.BytesToEd25519PrivateKey(ed25519PrivKeyStringBytes)
	//
	//decodedPrivKey, _ = crypto.BytesToEd25519PrivateKey(ed25519PrivKeyStringBytes)
	//decodedPubKey := make([]byte, ed25519.PublicKeySize)
	//copy(decodedPubKey, decodedPrivKey[ed25519.PublicKeySize:])
	//t.Log("decodedPrivKey.Seed: ", hex.EncodeToString(decodedPrivKey.Seed()))
	//t.Log("decodedPrivKey: ", hex.EncodeToString(decodedPrivKey))
	//t.Log("decodedPubKey: ", hex.EncodeToString(decodedPubKey))
	//agent := NewWithHost("http://127.0.0.1:8081", false, hex.EncodeToString(decodedPrivKey.Seed()))

	privKey, err := hex.DecodeHex(privKeyHexString)
	if err != nil {
		return
	}
	identity := identity.New(false, privKey)
	p := principal.NewSelfAuthenticating(identity.PubKeyBytes())
	t.Log("identity:", p.Encode(), len(identity.PubKeyBytes()))

	agent := NewWithHost("http://127.0.0.1:8081", false, privKeyHexString)
	t.Log("agent.identity.PubKey: ", hex.EncodeToString(agent.identity.PubKeyBytes()))
	p1 := principal.NewSelfAuthenticating(agent.identity.PubKeyBytes())
	t.Log("agent.identity: ", p1.Encode(), len(agent.identity.PubKeyBytes()))

	canister := "bw4dl-smaaa-aaaaa-qaacq-cai"
	methodName := "whoAmI"

	var argType []idl.Type
	//argType = append(argType, new(idl.Text))

	var argValue []interface{}
	//argValue = append(argValue, "world")

	arg, _ := idl.Encode(argType, argValue)
	types, result, str, err := agent.Query(canister, methodName, arg)
	if err != nil {
		panic(err)
	}
	t.Log(str, types[0].String(), " ->", principal.New(result[0].([]byte)).Encode())
}

func Test_local_RegisterNode(t *testing.T) {
	privKey, err := hex.DecodeHex(privKeyHexString)
	if err != nil {
		return
	}
	identity := identity.New(false, privKey)
	p := principal.NewSelfAuthenticating(identity.PubKeyBytes())
	t.Log("identity:", p.Encode(), len(identity.PubKeyBytes()))

	canister := "bw4dl-smaaa-aaaaa-qaacq-cai"
	agent := NewWithHost("http://127.0.0.1:8081", false, privKeyHexString)
	methodName := "registerNode"

	var argType []idl.Type
	argType = append(argType, new(idl.Nat))
	argType = append(argType, new(idl.Text))
	argType = append(argType, new(idl.Principal))

	argValue := []interface{}{
		big.NewInt(NodeComputing),
		"16Uiu2HAmG1a6Aqag9noiPnwB6y1SHnMYDP3mdJoZtKLSvDMTFp5v",
		p}
	arg, _ := idl.Encode(argType, argValue)
	t.Log("argType", argType)
	t.Log("argValue", argValue)
	t.Log(arg)

	types, result, err := agent.Update(canister, methodName, arg, 30)
	if err != nil {
		panic(err)
	}
	t.Log(types[0].String(), "\n ->", result)
	// (variant {Ok=0}) -> [map[17724:0 EnumIndex:17724]]
	// (variant {Err=variant {NodeAlreadyExist}})-> [map[3456837:map[440058177:<nil> EnumIndex:440058177] EnumIndex:3456837]]
}

func Test_local_ListNodes(t *testing.T) {
	canister := "bw4dl-smaaa-aaaaa-qaacq-cai"
	methodName := "listNodes"
	agent := NewWithHost("http://127.0.0.1:8081", false, privKeyHexString)

	var argType []idl.Type
	argType = append(argType, new(idl.Nat))
	argType = append(argType, new(idl.Nat))
	argType = append(argType, new(idl.Nat))

	argValue := []interface{}{
		big.NewInt(2),
		big.NewInt(0),
		big.NewInt(10)}
	arg, _ := idl.Encode(argType, argValue)
	t.Log("argType", argType)
	t.Log("argValue", argValue)
	t.Log(arg)

	types, result, str, err := agent.Query(canister, methodName, arg)
	if err != nil {
		panic(err)
	}
	t.Log(str, types[0].String(), "\n ->", result)
	// (vec {record {"16Uiu2HAm9kJriTsrV85UVnULzEqu7adzhxYg5wkUoMQc5PBYe5n3"; record {nodeID="16Uiu2HAm9kJriTsrV85UVnULzEqu7adzhxYg5wkUoMQc5PBYe5n3"; owner=principal "ov7pu-7jnok-6xopc-2vt5n-k7kmj-7mkjx-z2zo5-hudbj-6h3b7-ox4aq-rae"; lastActiveTime=1685353189081697488; wallet=principal "7aodp-4ebhh-pj5sa-5kdmg-fkkw3-wk6rv-yf4rr-pt2g7-ebx7j-7sjq4-4qe"; registered=1685353189081697488; nodeType=2}}})
	// ->
	// vec[
	// record[
	//		map[
	//			0:16Uiu2HAm9kJriTsrV85UVnULzEqu7adzhxYg5wkUoMQc5PBYe5n3
	//			1:map[
	//				1773021929:1685353189081697488
	//				3054210041:[129 57 222 158 200 29 80 216 98 169 86 221 149 232 215 5 228 98 249 232 223 32 111 244 254 73 135 57 2]
	//				4104166786:1685353189081697488
	//				4135997916:2
	//				656559709:16Uiu2HAm9kJriTsrV85UVnULzEqu7adzhxYg5wkUoMQc5PBYe5n3
	//				947296307:[45 114 189 119 60 90 172 250 213 125 76 79 216 164 223 58 203 186 122 12 41 241 246 31 186 252 4 34 2]
	//			]
	//		]
	//	]
	//]
	for _, record := range result[0].([]interface{}) {
		t.Log("record:", record)
	}
}

func Test_local_ListComputingNodes(t *testing.T) {
	canister := "bw4dl-smaaa-aaaaa-qaacq-cai"
	methodName := "listComputingNodes"

	privKeyHexString := "833fe62409237b9d62ec77587520911e9a759cec1d19755b7da901b96dca3d42"
	agent := NewWithHost("http://127.0.0.1:8080", false, privKeyHexString)

	var argType []idl.Type
	argType = append(argType, new(idl.Nat))
	argType = append(argType, new(idl.Nat))

	var argValue []interface{}
	argValue = append(argValue, big.NewInt(0))
	argValue = append(argValue, big.NewInt(10))

	//argValue := []interface{}{big.NewInt(0), big.NewInt(10)}
	arg, _ := idl.Encode(argType, argValue)
	t.Log("argType", argType)
	t.Log("argValue", argValue)
	t.Log(arg)

	types, result, str, err := agent.Query(canister, methodName, arg)
	if err != nil {
		panic(err)
	}
	t.Log(str, types[0].String(), "\n ->", result)
}