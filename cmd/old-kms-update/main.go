package main

import (
	"bytes"
	"context"
	"encoding/hex"
	j "encoding/json"
	"errors"
	"fmt"
	h "net/http"
	"os"

	"github.com/google/uuid"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
	"google.golang.org/grpc"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Println("Please provide a command: create-account, run-kms-update")
		return
	}

	if args[0] == "create-account" {
		// createAccount()
		fmt.Println("This command is not supported yet")
	} else if args[0] == "run-kms-update" {
		runKmsUpdate()
	} else {
		fmt.Println("Invalid command. Please provide a valid command: create-account, run-kms-update")
	}
}

const addAccountKeyTemplate = `
transaction(publicKey: String) {
	prepare(signer: auth(AddKey) &Account) {
		signer.addPublicKey(publicKey.decodeHex())
	}
}
`

func runKmsUpdate() {
	fmt.Println("Running KMS Update")
	ctx := context.Background()
	flowClient, err := client.New("127.0.0.1:3569", grpc.WithInsecure())
	examples.Handle(err)

	serviceAcctAddr, serviceAcctKey, _ := examples.ServiceAccount(flowClient)
	userAcct, getErr := flowClient.GetAccount(ctx, flow.HexToAddress("192440c99cb17282"))
	if getErr != nil {
		fmt.Println("Error getting account", getErr)
	}

	myPrivateKey := examples.RandomPrivateKey()
	myAcctKey := flow.NewAccountKey().
		FromPrivateKey(myPrivateKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flow.AccountKeyWeightThreshold)

	// addKeyTx := templates.AddAccountKey(userAcct.Address, myAcctKey)

	keyHex := myAcctKey.PublicKey.Encode()
	encodedStr := hex.EncodeToString(keyHex)
	fmt.Println("key hex: ", encodedStr)

	cadencePublicKey := cadence.String(keyHex)
	fmt.Println("cadence pub key: ", cadencePublicKey.String())

	addKeyTx := flow.NewTransaction().
		SetScript([]byte(addAccountKeyTemplate)).
		AddRawArgument(jsoncdc.MustEncode(cadencePublicKey)).
		AddAuthorizer(userAcct.Address)

	referenceBlockID := examples.GetReferenceBlockId(flowClient)

	addKeyTx.SetProposalKey(
		userAcct.Address, myAcctKey.Index, myAcctKey.SequenceNumber,
	)
	addKeyTx.SetReferenceBlockID(referenceBlockID)
	addKeyTx.SetPayer(serviceAcctAddr)

	txBody := SignTxRequestBody{
		Code: string(addKeyTx.Script),
		Arguments: []map[string]interface{}{
			{
				"type":  "String",
				"value": cadencePublicKey,
			},
		},
	}

	jobRes, jobErr := signTx(txBody, userAcct.Address.Hex())

	if jobErr != nil {
		fmt.Println("Error signing tx", jobErr)
	}

	fmt.Println("Job response: ", jobRes)
	addKeyTx.AddPayloadSignature(userAcct.Address, myAcctKey.Index, []byte(jobRes.PayloadSignatures[0].Signature))

	addKeyTx.AddEnvelopeSignature(serviceAcctAddr, serviceAcctKey.Index, []byte(jobRes.EnvelopeSignatures[0].Signature))

	// TODO: do this with the api instead??
	err = flowClient.SendTransaction(ctx, *addKeyTx)
	if err != nil {
		fmt.Println("Error sending transaction", err)
	}

	fmt.Println("Transaction sent")

	

}

type SignTxArguments []map[string]interface{}

type SignTxRequestBody struct {
	Code      string          `json:"code" binding:"required"`
	Arguments SignTxArguments `json:"arguments" binding:"required"`
}

type JobResponse struct {
	Code             string          `json:"code"`
	ReferenceBlockId string          `json:"referenceBlockId"`
	GasLimit         int          `json:"gasLimit"`
	ProposalKey      struct {
		Address        flow.Address `json:"address"`
		KeyIndex       int          `json:"keyIndex"`
		SequenceNumber int          `json:"sequenceNumber"`
	} `json:"proposalKey"`
	Payer             flow.Address   `json:"payer"`
	Authorizers       []flow.Address `json:"authorizers"`
	PayloadSignatures []struct {
		Address   flow.Address `json:"address"`
		KeyIndex  int          `json:"keyIndex"`
		Signature string       `json:"signature"`
	} `json:"payloadSignatures"`
	EnvelopeSignatures []struct {
		Address   flow.Address `json:"address"`
		KeyIndex  int          `json:"keyIndex"`
		Signature string       `json:"signature"`
	} `json:"envelopeSignatures"`
}

func signTx(reqBody SignTxRequestBody, acctAddr string) (JobResponse, error) {
	idempotencyKey := uuid.New().String()

	bodyAsJson, jsonErr := j.Marshal(reqBody)
	if jsonErr != nil {
		return JobResponse{}, jsonErr
	}
	fmt.Println("body as json: ", string(bodyAsJson))
	bodyReader := bytes.NewReader(bodyAsJson)

	req, reqErr := h.NewRequest("POST", "http://localhost:3005/v1/accounts/"+acctAddr+"/sign", bodyReader)
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

	fmt.Println("RESPONSE CODE: ", res.StatusCode)
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
