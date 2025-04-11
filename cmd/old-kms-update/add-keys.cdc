transaction(publicKey: String) {
	prepare(signer: auth(AddKey) &Account) {
		signer.addPublicKey(publicKey.decodeHex())
	}
}