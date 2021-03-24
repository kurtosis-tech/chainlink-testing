package genesis

// see the clique genesis json here: https://geth.ethereum.org/docs/interface/private-network
// must have extradata contain the default account, first funded. this is the default signer set for clique genesis
const GenesisJson =
	`{
    "config": {
		"chainId": 9,
		"homesteadBlock": 0,
		"eip150Block": 0,
		"eip155Block": 0,
		"eip158Block": 0,
		"byzantiumBlock": 0,
		"constantinopleBlock": 0,
		"petersburgBlock": 0,
		"clique": {
		  "period": 1,
		  "epoch": 3000
		}
	},
  	"difficulty": "1",
	"gasLimit": "10000000",
	"extradata": "0x00000000000000000000000000000000000000000000000000000000000000008ea1441a74ffbe9504a8cb3f7e4b7118d8ccfc560000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  	"alloc": {
    	"8ea1441a74ffbe9504a8cb3f7e4b7118d8ccfc56": { "balance": "30000000000000000000000000000000000000000000000000000" },
    	"6f75c1925ef6d0c9a23fba6e4b889c52dd9d7f74": { "balance": "30000000000000000000000000000000000000000000000000000" },
		"e68af577b1267c1e75d908668cb8ea4f72587d05": { "balance": "30000000000000000000000000000000000000000000000000000" }
	}
}`
