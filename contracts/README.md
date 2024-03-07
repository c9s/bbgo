# BBG Contracts
------------

### 1. Before Start

Create and modify the following files in this directory, the secret key inside the files are dummy ones from truffle dev server:
- development-secret.json
- polygon-secret.json 
- bsc-secret.json

### 2. Prepare the dependencies

```bash
npm i
# if you want to develop in localhost, try to run npm run devserver separately
# ex: npm run devserver
# it will give you a set of secrets and account addresses
```

### 3. Deploy

Migrate:
```bash
npm run migrate:dev
# npm run migrate:polygon
# npm run migrate:polygon-test
# npm run migrate:bsc
# npm run migrate:bsc-test
```

Lint:
```bash
npm run lint
# # fix solidity issue
# npm run lint:sol:fix
# # fix js issue
# npm run lint:js:fix
```

Test:
```bash
npm run test 
```

```bash
truffle run verify ChildMintableERC20 --network polygon
```

```bash
truffle run verify ChildMintableERC20@0x3Afe98235d680e8d7A52e1458a59D60f45F935C0 --network polygon
```
