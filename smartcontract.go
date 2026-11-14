package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SmartContract define el contrato inteligente para la gestión de votos en QoriChain
type SmartContract struct {
	contractapi.Contract
}

// Vote estructura los datos inmutables de cada voto emitido on-chain
type Vote struct {
	DocType    string `json:"docType"`
	Nullifier  string `json:"nullifier"`
	Ciphertext string `json:"ciphertext"`
	ZKP        string `json:"zkp"`
	Timestamp  string `json:"timestamp"`
}

// CastVote registra un voto cifrado previa validación estricta del Nullifier ("un ciudadano, un voto")
func (s *SmartContract) CastVote(ctx contractapi.TransactionContextInterface, nullifier string, ciphertext string, zkp string) error {
	if nullifier == "" {
		return fmt.Errorf("el nullifier no puede ser vacío")
	}
	if ciphertext == "" {
		return fmt.Errorf("el ciphertext no puede ser vacío")
	}

	voteKey := fmt.Sprintf("VOTE_%s", nullifier)

	// Verificación de integridad en el World State: Prevención de doble voto
	existingVoteBytes, err := ctx.GetStub().GetState(voteKey)
	if err != nil {
		return fmt.Errorf("error al verificar existencia previa del nullifier: %v", err)
	}
	if existingVoteBytes != nil && len(existingVoteBytes) > 0 {
		return fmt.Errorf("Doble voto detectado: el nullifier ya ha sido utilizado")
	}

	// Obtener marca de tiempo determinística de la transacción del peer
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	var formattedTimestamp string
	if err != nil || txTimestamp == nil {
		formattedTimestamp = time.Now().UTC().Format(time.RFC3339)
	} else {
		formattedTimestamp = time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos)).UTC().Format(time.RFC3339)
	}

	vote := Vote{
		DocType:    "vote",
		Nullifier:  nullifier,
		Ciphertext: ciphertext,
		ZKP:        zkp,
		Timestamp:  formattedTimestamp,
	}

	voteJSON, err := json.Marshal(vote)
	if err != nil {
		return fmt.Errorf("error al serializar el voto a JSON: %v", err)
	}

	// Persistencia atómica en el estado del ledger
	err = ctx.GetStub().PutState(voteKey, voteJSON)
	if err != nil {
		return fmt.Errorf("error al persistir el voto en el ledger: %v", err)
	}

	return nil
}

// QueryTally itera sobre los votos registrados en el ledger y extrae los Ciphertexts para suma homomórfica
func (s *SmartContract) QueryTally(ctx contractapi.TransactionContextInterface) ([]string, error) {
	// Rango de claves léxicas para capturar todas las entradas bajo el prefijo 'VOTE_'
	resultsIterator, err := ctx.GetStub().GetStateByRange("VOTE_", "VOTE_~")
	if err != nil {
		return nil, fmt.Errorf("error al ejecutar StateByRange de votos: %v", err)
	}
	defer resultsIterator.Close()

	var ciphertexts []string

	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("error al iterar sobre los resultados del ledger: %v", err)
		}

		var vote Vote
		err = json.Unmarshal(queryResponse.Value, &vote)
		if err != nil {
			return nil, fmt.Errorf("error al deserializar el registro de voto [%s]: %v", queryResponse.Key, err)
		}

		ciphertexts = append(ciphertexts, vote.Ciphertext)
	}

	return ciphertexts, nil
}
