var utils = require('./utils');
var initConfig = require('./config/initConfig.json');
var fs = require('fs');
var solc = require('solc')
var path = require('path')

var owner = {
    address: initConfig.baseaddr,
    privateKey: initConfig.privatekey
}

var subChain = initConfig.subChain;
var via = initConfig.vnodeVia;

var name = 'Coin for LBR';
var symbol = 'LTC';
var decimals = 18;
var totalSupply = 1000000000;

deploy(name, symbol, decimals, totalSupply);
return;

function deploy(name, symbol, decimals, totalSupply) {

    var contractName = 'StandardToken';
    var solpath = path.resolve(__dirname, "./contract/") + "/" + contractName + '.sol';
    var input = {
        language: 'Solidity',
        sources: {
            'StandardToken.sol': {
                content: fs.readFileSync(solpath, 'utf-8')
            }
        },
        settings: {
            optimizer: {
                enabled: true,
                runs: 200
            },
            outputSelection: {
                '*': {
                    '*': ['abi', 'evm.bytecode']
                }
            }
        }
    };

    var safeMathContract = fs.readFileSync(path.resolve(__dirname, "./contract/") + "/" + 'SafeMath.sol', 'utf-8');
    var IExtendedERC20Contract = fs.readFileSync(path.resolve(__dirname, "./contract/") + "/" + 'IExtendedERC20.sol', 'utf-8');

    function findImports(pathsol) {
        if (pathsol === 'SafeMath.sol')
            return { contents: safeMathContract };
        else if (pathsol == 'IExtendedERC20.sol')
            return { contents: IExtendedERC20Contract };
        else return { error: 'File not found' };
    }

    var output = JSON.parse(solc.compile(JSON.stringify(input), findImports));
    var abi = output.contracts[contractName + ".sol"][contractName].abi;
    var bin = output.contracts[contractName + ".sol"][contractName].evm.bytecode.object;

    var types = ['string', 'string', 'uint8', 'uint256'];
    var args = [name, symbol, decimals, totalSupply];
    var parameter = utils.chain3.encodeParams(types, args);

    var inNonce = utils.chain3.scs.getNonce(subChain, owner.address);
    console.log('nonce', inNonce);
    var rawTx = {
        from: owner.address,
        to: subChain,
        nonce: utils.chain3.toHex(inNonce),
        gasLimit: utils.chain3.toHex("0"),
        gasPrice: utils.chain3.toHex("0"),
        chainId: utils.chain3.toHex(utils.chain3.version.network),
        via: via,
        shardingFlag: "0x3",
        data: '0x' + bin + parameter
    };
    try {
        var signtx = utils.chain3.signTransaction(rawTx, owner.privateKey);
        var transHash = utils.chain3.mc.sendRawTransaction(signtx);
        console.log('txHash:', transHash);
        var tokenAddr = utils.waitBlockForContractInMicroChain(transHash);
        console.log('tokenAddr:', tokenAddr);
    } catch (error) {
        console.log(error)
    }
}
