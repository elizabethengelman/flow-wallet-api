import NonFungibleToken from 0xf8d6e0586b0a20c7
import Electables from 0xf8d6e0586b0a20c7
import Crypto

transaction(publicKeys: [Crypto.KeyListEntry], contracts: {String: String}) {
	prepare(signer: auth(Storage, Capabilities) &Account) {
		let acct = Account(payer: signer)

		for key in publicKeys {
            acct.keys.add(publicKey: key.publicKey, hashAlgorithm: key.hashAlgorithm, weight: key.weight)
		}

		for contract in contracts.keys {
			acct.contracts.add(name: contract, code: contracts[contract]!.decodeHex())
		}

        if acct.storage.borrow<&Electables.Collection>(from: Electables.CollectionStoragePath) == nil {            // create a new empty collection
           let collection <- Electables.createEmptyCollection(nftType: Type<@Electables.NFT>()) 

            // save it to the account
            acct.storage.save(<- collection, to: Electables.CollectionStoragePath)

            // Creates a public capability for the collection so that other users can publicly access electable attributes.
            // The pieces inside of the brackets specify the type of the linked object, and only expose the fields and
            // functions on those types.
            acct.capabilities.publish(
                acct.capabilities.storage.issue<&Electables.Collection>(Electables.CollectionStoragePath),
                at:Electables.CollectionPublicPath 
            )
        }
	}
}