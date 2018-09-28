var fs = require('fs');
var Web3 = require('web3');
const axios = require('axios');
const ethUtil = require('ethereumjs-util')
var web3 = new Web3();
web3 = new Web3(new Web3.providers.HttpProvider("http://localhost:8545"));

var url = "http://localhost:8080"
var owner = '85d6e595a3e64d3353b888bc49ee27f1b9f2a656'
var amount = 10
var privatekey = "7c258dd1847a33ccc7691ebf3fd83ec0575bbd4ffb0f1f3524ac45d9291f6fd3"
axios.defaults.baseURL = url

function deposit() {
  axios.post('/deposit', {
    owner: owner,
	amount: amount
  })
  .then(function (response) {
    console.log(response.data.txhash);

    var sig = sign(response.data.txhash)
    afterSign(sig)
  })
  .catch(function (error) {
    console.log(error);
  });
}

function sign(txHash) {
    var hashBuf = new Buffer(txHash.substr(2), "hex")
    var pkBuf = new Buffer(privatekey, "hex")
    // var sig = web3.eth.sign(owner, hash)
    console.log(hashBuf.length);
    console.log(pkBuf.length);
    var sig = ethUtil.ecsign(hashBuf, pkBuf)
    sigHex = ethUtil.toRpcSig(sig.v, sig.r, sig.s)
    console.log("sig:", sigHex);
    return sigHex
}

function afterSign(sig) {
  axios.post('/aftersign', {
    owner: owner,
	amount: amount,
    sig: sig
  })
  .then(function (response) {
    console.log(response.data.res);
  })
  .catch(function (error) {
    console.log(error.Error);
  });
}

deposit();
