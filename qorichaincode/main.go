package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	qoriContract := new(SmartContract)
	cc, err := contractapi.NewChaincode(qoriContract)
	if err != nil {
		log.Panicf("Error instanciando QoriChaincode: %v", err)
	}

	if err := cc.Start(); err != nil {
		log.Panicf("Error iniciando ejecución de QoriChaincode: %v", err)
	}
}
