let ethers = require('ethers')

let ETH_RPC_URL="http://172.23.0.5:8545"
let PRIVATE_KEY_JSON_PASSWORD = "password"
let ORACLE_CONTRACT_ADDRESS='0x4758E84AbAD42355454fC85cdED2e64A82ad15E0'
let ORACLE_ETHEREUM_ADDRESS='0xaDE5c9d2D994a729AF54FEd9e8b84d05727e19e2'
let PRIVATE_KEY_JSON = {"address":"8ea1441a74ffbe9504a8cb3f7e4b7118d8ccfc56","crypto":{"cipher":"aes-128-ctr","ciphertext":"2dfb66792b39f458365f8604e959d000a57a44c5c9e935130da75edb21571666","cipherparams":{"iv":"c75546ec881dcd668e7d9cb4f75d24f3"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"4cb212065dfaba68e7a2e99f42d2bf4e10edc5793390424bfeb4c73a381dbdfd"},"mac":"98c469923b668bd1655e8acdb40b7d9d5ceae53058b5fd706064595d10b67142"},"id":"f64bbf7e-e34f-442e-91b9-9bc0a1190edf","version":3}
let json = JSON.stringify(PRIVATE_KEY_JSON)


let provider = new ethers.providers.JsonRpcProvider(ETH_RPC_URL)
let oracleAddress = ORACLE_CONTRACT_ADDRESS
let oracleAbi = [
    {
        "constant": false,
        "inputs": [
            {
                "name": "_sender",
                "type": "address"
            },
            {
                "name": "_payment",
                "type": "uint256"
            },
            {
                "name": "_specId",
                "type": "bytes32"
            },
            {
                "name": "_callbackAddress",
                "type": "address"
            },
            {
                "name": "_callbackFunctionId",
                "type": "bytes4"
            },
            {
                "name": "_nonce",
                "type": "uint256"
            },
            {
                "name": "_dataVersion",
                "type": "uint256"
            },
            {
                "name": "_data",
                "type": "bytes"
            }
        ],
        "name": "oracleRequest",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "_requestId",
                "type": "bytes32"
            },
            {
                "name": "_payment",
                "type": "uint256"
            },
            {
                "name": "_callbackAddress",
                "type": "address"
            },
            {
                "name": "_callbackFunctionId",
                "type": "bytes4"
            },
            {
                "name": "_expiration",
                "type": "uint256"
            },
            {
                "name": "_data",
                "type": "bytes32"
            }
        ],
        "name": "fulfillOracleRequest",
        "outputs": [
            {
                "name": "",
                "type": "bool"
            }
        ],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [],
        "name": "EXPIRY_TIME",
        "outputs": [
            {
                "name": "",
                "type": "uint256"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [],
        "name": "withdrawable",
        "outputs": [
            {
                "name": "",
                "type": "uint256"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "_requestId",
                "type": "bytes32"
            },
            {
                "name": "_payment",
                "type": "uint256"
            },
            {
                "name": "_callbackFunc",
                "type": "bytes4"
            },
            {
                "name": "_expiration",
                "type": "uint256"
            }
        ],
        "name": "cancelOracleRequest",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [],
        "name": "renounceOwnership",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "_node",
                "type": "address"
            },
            {
                "name": "_allowed",
                "type": "bool"
            }
        ],
        "name": "setFulfillmentPermission",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [],
        "name": "owner",
        "outputs": [
            {
                "name": "",
                "type": "address"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "_sender",
                "type": "address"
            },
            {
                "name": "_amount",
                "type": "uint256"
            },
            {
                "name": "_data",
                "type": "bytes"
            }
        ],
        "name": "onTokenTransfer",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [
            {
                "name": "_node",
                "type": "address"
            }
        ],
        "name": "getAuthorizationStatus",
        "outputs": [
            {
                "name": "",
                "type": "bool"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "_newOwner",
                "type": "address"
            }
        ],
        "name": "transferOwnership",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "constant": false,
        "inputs": [
            {
                "name": "_recipient",
                "type": "address"
            },
            {
                "name": "_amount",
                "type": "uint256"
            }
        ],
        "name": "withdraw",
        "outputs": [],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "function"
    },
    {
        "inputs": [
            {
                "name": "_link",
                "type": "address"
            }
        ],
        "payable": false,
        "stateMutability": "nonpayable",
        "type": "constructor"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": true,
                "name": "specId",
                "type": "bytes32"
            },
            {
                "indexed": false,
                "name": "requester",
                "type": "address"
            },
            {
                "indexed": false,
                "name": "requestId",
                "type": "bytes32"
            },
            {
                "indexed": false,
                "name": "payment",
                "type": "uint256"
            },
            {
                "indexed": false,
                "name": "callbackAddr",
                "type": "address"
            },
            {
                "indexed": false,
                "name": "callbackFunctionId",
                "type": "bytes4"
            },
            {
                "indexed": false,
                "name": "cancelExpiration",
                "type": "uint256"
            },
            {
                "indexed": false,
                "name": "dataVersion",
                "type": "uint256"
            },
            {
                "indexed": false,
                "name": "data",
                "type": "bytes"
            }
        ],
        "name": "OracleRequest",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": true,
                "name": "requestId",
                "type": "bytes32"
            }
        ],
        "name": "CancelOracleRequest",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": true,
                "name": "previousOwner",
                "type": "address"
            }
        ],
        "name": "OwnershipRenounced",
        "type": "event"
    },
    {
        "anonymous": false,
        "inputs": [
            {
                "indexed": true,
                "name": "previousOwner",
                "type": "address"
            },
            {
                "indexed": true,
                "name": "newOwner",
                "type": "address"
            }
        ],
        "name": "OwnershipTransferred",
        "type": "event"
    }
]

let gasPrice = 30000000000
const main = async () => {
    let overrides = {
        gasPrice: gasPrice
    }
    let address = ORACLE_ETHEREUM_ADDRESS
    ethers.Wallet.fromEncryptedJson(json, PRIVATE_KEY_JSON_PASSWORD).then(async function(wallet) {
        wallet = wallet.connect(provider)
        let oracle = new ethers.Contract(oracleAddress, oracleAbi, wallet)
        let contractWithSigner = oracle.connect(wallet)
        // set the address here to the node wallet address (regular address not emergency)
        let tx = await contractWithSigner.setFulfillmentPermission(address, true, overrides)
        await tx.wait()
        let authStatus = await contractWithSigner.getAuthorizationStatus(address)
        console.log(authStatus)
    })
}

main()