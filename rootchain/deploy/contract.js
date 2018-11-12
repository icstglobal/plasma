var solc = require('solc');
var fs = require('fs');
var Web3 = require('web3');
var web3 = new Web3();
web3 = new Web3(new Web3.providers.HttpProvider("http://localhost:8545"));


var cxAddr = "0xf9f927ec90320c257b0ba5a597c50c175a155c14"
var dir = "contract_data"
function loadContract(name) {
    var data = fs.readFileSync(`${dir}/${name}.json`)
    data = JSON.parse(data.toString())

    var cxFactory = web3.eth.contract(data.abi)
    return cxFactory.at(cxAddr)
}

cx = loadContract("RootChain")
module.exports = cx;
