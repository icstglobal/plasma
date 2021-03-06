var solc = require('solc');
var fs = require('fs');
var Web3 = require('web3');
var web3 = new Web3();
web3 = new Web3(new Web3.providers.HttpProvider("http://119.28.248.204:8546"));

var dir = "contract_data"

function compile() {
    var input = {
    }

    var path = "../contracts/"

    var items = fs.readdirSync(path) 
    for (var i=0; i<items.length; i++) {
        if (!items[i].endsWith("sol")){
            continue
        }
        var data = fs.readFileSync(path+items[i])
        input[items[i]] = data.toString()
    }

    var output = solc.compile({ sources: input }, 1)

    console.log(output);
    if (!fs.existsSync(dir)){
        fs.mkdirSync(dir);
    }

    for (var contractName in output.contracts){
        var name = contractName.split(".")[0]
        var data = {}
        data.abi = JSON.parse(output.contracts[contractName].interface)
        data.bytecode = "0x"+output.contracts[contractName].bytecode
        fs.writeFileSync(`${dir}/${name}.json`, JSON.stringify(data))
    }
}

function deploy(name) {
    var data = fs.readFileSync(`${dir}/${name}.json`)
    data = JSON.parse(data.toString())

    var cxFactory = web3.eth.contract(data.abi)

    console.log(web3.eth.accounts[0], web3.eth.blockNumber);
    var contract = cxFactory.new({from:web3.eth.accounts[0],data:data.bytecode,gas:3000000}, function (error, cx) {
        console.log(error, cx)
    })
}

var args = process.argv.slice(2);

switch (args[0]) {
    case 'compile':
        compile();
        break;
    case 'deploy':
        deploy("RootChain");
        break;
    default:
        compile();
        deploy("RootChain");
}
