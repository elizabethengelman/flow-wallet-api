package main

import (
	"bytes"
	"context"
	j "encoding/json"
	"errors"
	"fmt"
	h "net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/access/http"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go-sdk/examples"

	"github.com/onflow/flowkit/config"
	"github.com/onflow/flowkit/config/json"
	"github.com/spf13/afero"
)

const configPath = "./flow.json"

var Config *config.Config

func initConfig() {
	var mockFS = afero.NewOsFs()

	var af = afero.Afero{Fs: mockFS}

	l := config.NewLoader(af)

	l.AddConfigParser(json.NewParser())

	var err error
	Config, err = l.Load([]string{configPath})
	if err != nil {
		fmt.Println("Error loading config", err)
		return
	}
}

func main() {
	initConfig()
	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Println("Please provide a command: create-account, run-kms-update")
		return
	}

	if args[0] == "create-account" {
		createAccount()
	} else if args[0] == "run-kms-update" {
		runKmsUpdate()
	} else {
		fmt.Println("Invalid command. Please provide a valid command: create-account, run-kms-update")
	}
}

func createAccount() {
	fmt.Println("Creating a new account")
	ctx := context.Background()
	flowClient, err := http.NewClient(http.EmulatorHost)
	if err != nil {
		fmt.Println("Error creating the flow client", err)
	}

	serviceAcctAddr, serviceAcctKey, serviceSigner := ServiceAccount(flowClient)

	fmt.Printf("flow client %v %v", ctx, flowClient)

	myPrivateKey := examples.RandomPrivateKey()
	fmt.Println("my private key: ", myPrivateKey)
	fmt.Println("hash algo: ", crypto.SHA3_256)
	myAcctKey := flow.NewAccountKey().
		FromPrivateKey(myPrivateKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flow.AccountKeyWeightThreshold)

	referenceBlockID := examples.GetReferenceBlockId(flowClient)
	createAccountTx, err := templates.CreateAccount([]*flow.AccountKey{myAcctKey}, nil, serviceAcctAddr)
	examples.Handle(err)
	createAccountTx.SetProposalKey(
		serviceAcctAddr,
		serviceAcctKey.Index,
		serviceAcctKey.SequenceNumber,
	)
	createAccountTx.SetReferenceBlockID(referenceBlockID)
	createAccountTx.SetPayer(serviceAcctAddr)

	// // Sign the transaction with the service account, which already exists
	// // All new accounts must be created by an existing account
	err = createAccountTx.SignEnvelope(serviceAcctAddr, serviceAcctKey.Index, serviceSigner)
	if err != nil {
		fmt.Println("Error signing envelope", err)
	}

	// // Send the transaction to the network
	err = flowClient.SendTransaction(ctx, *createAccountTx)
	if err != nil {
		fmt.Println("Error sending transaction", err)
	}

	accountCreationTxRes := examples.WaitForSeal(ctx, flowClient, createAccountTx.ID())

	fmt.Println("create accoutn tx id: ", createAccountTx.ID())

	var myAddress flow.Address

	fmt.Printf("acct create tx res events %v", accountCreationTxRes.Events)

	for _, event := range accountCreationTxRes.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			myAddress = accountCreatedEvent.Address()
		}
	}

	fmt.Println("Account created with address:", myAddress.Hex())
	// and for just setting this up... i can call the flow-wallet-api?

}

func ServiceAccount(flowClient access.Client) (flow.Address, *flow.AccountKey, crypto.Signer) {
	acc := Config.Accounts[0]
	privateKey, err := crypto.DecodePrivateKeyHex(crypto.ECDSA_P256, strings.Replace(acc.Key.PrivateKey.String(), "0x", "", 1))
	if err != nil {
		fmt.Println("Error decoding private key", err)
	}

	account, err := flowClient.GetAccount(context.Background(), acc.Address)
	if err != nil {
		fmt.Printf("failed to get account %s", err)
	}
	accountKey := account.Keys[0]
	signer, err := crypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)
	if err != nil {
		fmt.Printf("Error creating in memory signer %s", err)
	}
	return flow.HexToAddress(acc.Address.Hex()), accountKey, signer
}


func runKmsUpdate() {
	fmt.Println("Running KMS update")

	ctx := context.Background()

	flowClient, err := http.NewClient(http.EmulatorHost)
	examples.Handle(err)

	// instead of Random Account, get an account ... 192440c99cb17282
	acct, getErr := flowClient.GetAccount(ctx, flow.HexToAddress("192440c99cb17282"))
	if getErr != nil {
		fmt.Println("Error getting account", getErr)
	}

	acctAddr := flow.HexToAddress("192440c99cb17282")
	acctKey := acct.Keys[0]

	// acctAddr, acctKey, acctSigner := examples.RandomAccount(flowClient)

	fmt.Println("Account address:", acctAddr)
	fmt.Println("Accoutn Key: ", acctKey)

	// Create the new key to add to your account
	myPrivateKey := examples.RandomPrivateKey()
	myAcctKey := flow.NewAccountKey().
		FromPrivateKey(myPrivateKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flow.AccountKeyWeightThreshold)

	addKeyTx, err := templates.AddAccountKey(acctAddr, myAcctKey)
	examples.Handle(err)

	// referenceBlockID := examples.GetReferenceBlockId(flowClient)
	
	// serviceAcctAddr,  serviceAcctKey, serviceSigner := ServiceAccount(flowClient)


	// addKeyTx.SetProposalKey(acctAddr, acctKey.Index, acctKey.SequenceNumber)
	// addKeyTx.SetReferenceBlockID(referenceBlockID)

	// addKeyTx.SetPayer(serviceAcctAddr)
	// addKeyTx.AddAuthorizer(acctAddr)



	// actually, i don't think i need to do this, because that is how flow-wallet-api is already setup 
	// we just need the account to be the proposer?

	// i hope this signing functionality isnt too outdated :|
	keyAsKeyListEntry, kErr := templates.AccountKeyToCadenceCryptoKey(acctKey)
	if kErr != nil {
		fmt.Println("Error converting account key to cadence crypto key", kErr)
	}

	txBody := SignTxRequestBody{
		Code: string(addKeyTx.Script),
		Arguments: []map[string]interface{}{
			{
				"type":  "Crypto.KeyListEntry",
				"value": keyAsKeyListEntry,
			},
		},
	}

	jobRes, jobErr := signTx(txBody, acctAddr.Hex())
	 
	if jobErr != nil {
		fmt.Println("Error signing tx", jobErr)
	}
	
	fmt.Println("Job response: ", jobRes)

		// instead of this... i think i need to send the tx the flow-wallet-api to have it signed by the account
		// err = addKeyTx.SignPayload(acctAddr, acctKey.Index, accountASigner)
		// if err != nil {
		// 	panic(fmt.Sprintf("Failed to sign as Account A: %v", err))
		// }
	

	// // Send the transaction to the network.
	// err = flowClient.SendTransaction(ctx, *addKeyTx)
	// examples.Handle(err)

	// examples.WaitForSeal(ctx, flowClient, addKeyTx.ID())

	// fmt.Println("Public key added to account!")

}


type JobResponse struct {
	JobId         string   `json:"jobId"`
	Type          string   `json:"type"`
	State         string   `json:"state"`
	Error         string   `json:"error"`
	Errors        []string `json:"errors"`
	Result        string   `json:"result"`
	TransactionId string   `json:"transactionId"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

type SignTxArguments []map[string]interface{}

type SignTxRequestBody struct {
	Code string `json:"code" binding:"required"`
	Arguments SignTxArguments `json:"arguments" binding:"required"`
}

func signTx(reqBody SignTxRequestBody, acctAddr string) (JobResponse, error) {
	idempotencyKey := uuid.New().String()

	bodyAsJson, jsonErr := j.Marshal(reqBody)
	if jsonErr != nil {
		return JobResponse{}, jsonErr
	}
	fmt.Println("body as json: ", string(bodyAsJson))
	bodyReader := bytes.NewReader(bodyAsJson)

	req, reqErr := h.NewRequest("POST", "http://localhost:3005/v1/accounts/" + acctAddr + "/sign", bodyReader)
	if reqErr != nil {
		return JobResponse{}, reqErr
	}

	req.Header.Add("Idempotency-Key", idempotencyKey)
	req.Header.Add("Content-Type", "application/json")

	httpClient := &h.Client{}

	res, resErr := httpClient.Do(req)
	if resErr != nil {
		return JobResponse{}, resErr
	}

	if res.StatusCode != h.StatusCreated {
		fmt.Println("status code: ", res.StatusCode)
		return JobResponse{}, errors.New("failed to sign transaction")
	}

	var body JobResponse
	decodeErr := j.NewDecoder(res.Body).Decode(&body)
	if decodeErr != nil {
		return JobResponse{}, decodeErr
	}

	return body, nil
}


/*
Next steps:
- for now, just deal with the manually inserted storable keys
	- create the account using cmd/kms-update/main.go
- need to make sure im encrypting the pk correctly when storing it in the db... not doing this right now

- write the update keys function
- test it

- let karan know the plan
- need to update the flow api because it is kind of broken



some test accounts to add to the db:
add: e03daebed8ca0615
pk: 0xf3da19d2cb1c65478844272b706e73c48b52a8a4376867f62c1d13ce5d09a34c
hash_algo: SHA3_256
sign_algo: ECDSA_P256
INSERT INTO storable_keys (account_address, index, type, value, sign_algo, hash_algo, public_key)
VALUES ('e03daebed8ca0615', 0, 'local', 'f3da19d2cb1c65478844272b706e73c48b52a8a4376867f62c1d13ce5d09a34c', 'ECDSA_P256', 'SHA3_256', '1f85663f5b99be3a1546929303570fb68da107c3acc5b1c500b84447322c89a256b1f3f3728acab38a5587d90da446a7890b0887a324860acb9297989d9f3920');


045a1763c93006ca
0x24cba0826d219d0ddb00ecb1e36f1a8207ee4434f50350e2e7478ec568c85dfd
INSERT INTO storable_keys (account_address, index, type, value, sign_algo, hash_algo, public_key)
Values('045a1763c93006ca', 0, 'local', '24cba0826d219d0ddb00ecb1e36f1a8207ee4434f50350e2e7478ec568c85dfd', 'ECDSA_P256', 'SHA3_256', '8b003307078dbda90667c6dfb114f35dc0b5f3590516cb0cd380fdb443ed5bfae959e1b83f51a86dd8fec77d60f88c19d4c7b639c02a337b5c8fc467200b53b2');

120e725050340cab
0x65a544b6eff754e67b4f7a4e2c47850d068145eb2eab09ac524cb138b39c083d
INSERT INTO storable_keys (account_address, index, type, value, sign_algo, hash_algo, public_key)
values('120e725050340cab', 0, 'local', '65a544b6eff754e67b4f7a4e2c47850d068145eb2eab09ac524cb138b39c083d', 'ECDSA_P256', 'SHA3_256', 'a08e8610062caa344ab5acbd80076cf52a6943374949c2fc525666a97c52d61e6e955cf5e1841775b3c6370dfecf359b2cb5f644b04e5a0bdf27c9ab3484d829');

0x9758b7cd0fc42c84f017c4dd648421f522b7ba831b232f6d9162e7f0574d726c
192440c99cb17282

add: 192440c99cb17282
pk: 0x9758b7cd0fc42c84f017c4dd648421f522b7ba831b232f6d9162e7f0574d726c
hash_algo: SHA3_256
sign_algo: ECDSA_P256
INSERT INTO storable_keys (account_address, index, type, value, sign_algo, hash_algo, public_key)
VALUES ('192440c99cb17282', 0, 'local', '9758b7cd0fc42c84f017c4dd648421f522b7ba831b232f6d9162e7f0574d726', 'ECDSA_P256', 'SHA3_256', 'dcea3af2807db097b663bbc019d436120c81889a363511741cffdac727a879d16ce5c4dec72ffada65b9dfaf76ec13db5e1c4a00955a6e7ec86b81387979a2b4');


INSERT INTO storable_keys (account_address, index, type, value, sign_algo, hash_algo, public_key) Values('f8d6e0586b0a20c7', 0, 'local', 'bc075f150b041e732625a60e94cb15ac1cafa8510c2a44d09d76ff5165df2b7b', 'ECDSA_P256', 'SHA3_256', '5908ab782480a80b8cfc6ae3de6c1ec6d6fcd28c9cd1b6091b70029cd4a40bc538eb6fe0b8f99b1d21af640bc48c602910cb88c6cf4a88a5cb8a4e0efe56ac63');



”t pk from `flow keys derive <pk>` on the cli
*/

