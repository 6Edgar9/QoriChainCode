# Análisis Técnico y Arquitectura de Código: QoriChaincode

Este documento ofrece un desglose exhaustivo de la arquitectura de software, fundamentos criptográficos, flujos de ejecución y detalles de implementación del contrato inteligente **QoriChaincode** para **Hyperledger Fabric v2.5**.

---

## 1. Fundamentos Criptográficos y de Negocio

El objetivo de **QoriChaincode** es habilitar elecciones electrónicas inmutables, anónimas y verificables. Resuelve dos desafíos esenciales en los sistemas descentralizados:

```
[ Votante ]
    │
    ├── Genera Nullifier = Hash(Secret_Votante, Election_ID)
    ├── Cifra Voto con Paillier = Enc(Pub_Key, Opcion) -> Ciphertext
    └── Genera Prueba ZKP (voto en rango legítimo)
    │
    ▼ Invoca CastVote(nullifier, ciphertext, zkp)
[ Hyperledger Fabric Peer ]
    │
    ├── Verifica World State: ¿Existe 'VOTE_<nullifier>'?
    │       ├── SI -> Error: "Doble voto detectado"
    │       └── NO -> Registra Vote{...} en World State
    │
    ▼ Fin de Elección: Invocación QueryTally()
[ QoriCore (Tally Engine) ]
    │
    ├── Recibe: [c1, c2, c3, ..., cN]
    └── Calcula Homomórficamente: C_total = ∏ ci mod n²
```

### 1.1 Esquema de Nullifiers (Prevención de Doble Voto)
En un sistema de votación tradicional, la identidad del votante se desvincula de la boleta tras depositarla en la urna. En blockchain, para evitar que una persona vote dos veces sin revelar quién es, se utiliza un **Nullifier**:
* Se trata de un identificador criptográfico pseudo-anónimo y determinístico derivado de la identidad del votante y el identificador de la elección:
  $$\text{Nullifier} = \mathcal{H}(\text{Secreto Privado del Votante} \parallel \text{ID de la Elección})$$
* El chaincode almacena los registros con la clave de estado `VOTE_<nullifier>`.
* Antes de aceptar cualquier voto, el chaincode consulta el *World State*. Si la clave ya contiene datos, la transacción es rechazada automáticamente con el error de **"Doble voto detectado"**.

### 1.2 Criptosistema Homomórfico de Paillier
El sistema de Paillier es aditivamente homomórfico:
* Dados dos textos cifrados $c_1 = E(m_1)$ y $c_2 = E(m_2)$, su producto modular corresponde al cifrado de la suma de los mensajes:
  $$c_{\text{sum}} = (c_1 \cdot c_2) \pmod{n^2} = E(m_1 + m_2 \pmod{n})$$
* Durante la fase de votación, los votos se almacenan cifrados (`Ciphertext`) en el ledger. Nadie (ni siquiera los nodos endorsers u administradores de la red) puede ver la opción seleccionada.
* En la fase de conteo, `QueryTally` extrae todos los `Ciphertexts`, y el módulo **QoriCore** multiplica los textos cifrados para obtener el cifrado de la suma total, descifrando únicamente el resultado consolidado final.

---

## 2. Desglose del Código Fuente

### 2.1 `main.go`: Punto de Entrada y Ciclo de Vida

```go
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
```

* **`new(SmartContract)`**: Instancia el receptor de métodos de nuestro contrato.
* **`contractapi.NewChaincode(qoriContract)`**: Inicializa el runtime del SDK de Fabric (`fabric-contract-api-go`). Este paquete envuelve las primitivas de bajo nivel (`shim.ChaincodeStubInterface`) en una interfaz de alto nivel basada en métodos públicos exportados.
* **`cc.Start()`**: Establece la conexión gRPC bidireccional permanente entre el proceso del chaincode y el peer de Hyperledger Fabric (manejando las llamadas de `Invoke`, `Query` y las negociaciones del ciclo de vida).

---

### 2.2 `smartcontract.go`: Lógica de Negocio y Persistencia

#### Estructura del SmartContract y del Modelo `Vote`
```go
type SmartContract struct {
	contractapi.Contract
}

type Vote struct {
	DocType    string `json:"docType"`
	Nullifier  string `json:"nullifier"`
	Ciphertext string `json:"ciphertext"`
	ZKP        string `json:"zkp"`
	Timestamp  string `json:"timestamp"`
}
```

* **Herencia por composición**: `SmartContract` embebe `contractapi.Contract`, otorgándole las capacidades de ciclo de vida e inspección de metadatos del Fabric Contract API.
* **`Vote`**:
  * `DocType`: Clasificador de documento (`"vote"`), útil para consultas ricas con CouchDB.
  * `Nullifier`: El hash único del votante.
  * `Ciphertext`: El valor cifrado con la clave pública de Paillier.
  * `ZKP`: La prueba de conocimiento cero serializada en base64 o hex.
  * `Timestamp`: Fecha y hora de emisión del voto en formato RFC3339.

---

#### Método `CastVote`: Validación y Registro
```go
func (s *SmartContract) CastVote(ctx contractapi.TransactionContextInterface, nullifier string, ciphertext string, zkp string) error
```

1. **Validación de Parámetros**:
   ```go
   if nullifier == "" {
       return fmt.Errorf("el nullifier no puede ser vacío")
   }
   if ciphertext == "" {
       return fmt.Errorf("el ciphertext no puede ser vacío")
   }
   ```
   Evita inserciones nulas o estados corruptos en el ledger.

2. **Detección de Doble Voto en el World State**:
   ```go
   voteKey := fmt.Sprintf("VOTE_%s", nullifier)
   existingVoteBytes, err := ctx.GetStub().GetState(voteKey)
   if err != nil {
       return fmt.Errorf("error al verificar existencia previa del nullifier: %v", err)
   }
   if existingVoteBytes != nil && len(existingVoteBytes) > 0 {
       return fmt.Errorf("Doble voto detectado: el nullifier ya ha sido utilizado")
   }
   ```
   * `ctx.GetStub().GetState(voteKey)` realiza una lectura en el estado del canal.
   * Si la clave ya tiene datos (`len > 0`), se aborta de inmediato. Esto garantiza que dentro de la fase de endorsment y simulación de Fabric, si dos transacciones con el mismo nullifier entran en el mismo bloque, la segunda fallará por validación MVCC (Multi-Version Concurrency Control) en el commit phase del peer.

3. **Determinismo Temporal**:
   ```go
   txTimestamp, err := ctx.GetStub().GetTxTimestamp()
   ```
   * **Importancia crítica en Blockchain**: Si se usase directamente `time.Now()` de Go, cada peer endorser obtendría una marca de tiempo diferente, produciendo un *Read-Write Set* inconsistente que provocaría el rechazo de la transacción por falta de consenso.
   * `ctx.GetStub().GetTxTimestamp()` obtiene la marca temporal incluida en la cabecera del mensaje de la propuesta de transacción (`ChannelHeader`), la cual es idéntica en todos los peers.

4. **Persistencia Atómica**:
   ```go
   voteJSON, err := json.Marshal(vote)
   // ...
   err = ctx.GetStub().PutState(voteKey, voteJSON)
   ```
   Se serializa la estructura `Vote` a JSON y se persiste con `PutState`.

---

#### Método `QueryTally`: Extracción para Agregación Homomórfica
```go
func (s *SmartContract) QueryTally(ctx contractapi.TransactionContextInterface) ([]string, error)
```

1. **Rango Léxico de Claves**:
   ```go
   resultsIterator, err := ctx.GetStub().GetStateByRange("VOTE_", "VOTE_~")
   defer resultsIterator.Close()
   ```
   * En LevelDB y CouchDB, las claves se ordenan lexicográficamente.
   * El carácter `~` tiene un valor ASCII (`0x7E`) mayor que números y letras, lo que garantiza que `"VOTE_~"` delimite superiormente cualquier clave que inicie con `"VOTE_"`.
   * `defer resultsIterator.Close()` libera los recursos del cursor iterador del motor de base de datos.

2. **Extracción y Deserialización**:
   ```go
   for resultsIterator.HasNext() {
       queryResponse, err := resultsIterator.Next()
       var vote Vote
       json.Unmarshal(queryResponse.Value, &vote)
       ciphertexts = append(ciphertexts, vote.Ciphertext)
   }
   ```
   * Recorre secuencialmente todos los registros de voto y construye un array que contiene **exclusivamente** los `Ciphertexts`.
   * Esto minimiza el consumo de ancho de banda y la sobrecarga computacional de la red, devolviendo la carga útil exacta requerida por el acumulador de **QoriCore**.

---

## 3. Dockerfile.build y Compilación Aislada

```dockerfile
FROM golang:1.20-alpine

RUN apk add --no-cache git build-base

WORKDIR /app

COPY . .

RUN go mod init qorichaincode && \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -v -ldflags="-s -w" -o chaincode .

CMD ["./chaincode"]
```

* **Imagen Base**: `golang:1.20-alpine` proporciona un entorno reproducible y ligero.
* **`git` y `build-base`**: Requeridos para que `go mod tidy` resuelva las dependencias externas desde repositorios git.
* **`CGO_ENABLED=0`**: Deshabilita dependencias dinámicas de librerías C, generando un binario puramente estático que puede ejecutarse dentro de cualquier contenedor peer de Fabric sin problemas de `glibc` o librerías compartidas.
* **`-ldflags="-s -w"`**: Remueve la tabla de símbolos y la información de depuración DWARF, reduciendo drásticamente el tamaño del binario resultante.

---

## 4. Flujo de Ejecución del Script `build_qorichaincode.sh`

1. **Generación con Here-Docs (`cat << 'EOF'`)**: Genera los archivos directamente en la raíz de `QoriChainCode` sin crear subcarpetas redundantes.
2. **Contexto Limpio**: Usa `.dockerignore` para enviar al contexto de construcción de Docker únicamente los archivos fuente relevantes.
3. **Construcción en Docker**: Ejecuta `docker build`, validando en tiempo real la descarga del SDK de Fabric y la compilación exitosa.
4. **Idempotencia y Limpieza**: Al terminar la validación, elimina la imagen generada (`qorichaincode-builder`) para no dejar residuos en el disco local.
