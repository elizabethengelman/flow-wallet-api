transaction(greeting: String) {
  let guest: Address

  prepare(authorizer: &Account) {
    self.guest = authorizer.address
  }

  execute {
    log(greeting.concat(",").concat(self.guest.toString()))
  }
}
