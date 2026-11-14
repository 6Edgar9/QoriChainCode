# QoriChaincode: Contrato Inteligente de Votación Descentralizada

[![Hyperledger Fabric](https://img.shields.io/badge/Hyperledger%20Fabric-v2.5-blue.svg)](https://hyperledger-fabric.readthedocs.io/)
[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)
[![Cryptography](https://img.shields.io/badge/Crypto-Paillier%20%2B%20ZKP-orange.svg)]()

**QoriChaincode** es el contrato inteligente (*smart contract*) on-chain central para el ecosistema de votación electrónica segura y descentralizada **QoriChain**. Diseñado para ejecutarse sobre **Hyperledger Fabric v2.5**, este chaincode implementa mecanismos criptográficos para garantizar el principio democrático fundamental de **"un ciudadano, un voto"** manteniendo la privacidad absoluta de los sufragios mediante el criptosistema homomórfico de **Paillier** y pruebas de conocimiento cero (**ZKP**).

---

## 🏛️ Características Principales

1. **Garantía "Un Ciudadano, Un Voto" (Nullifier Scheme)**:
   - Registro de un identificador determinístico pseudo-anónimo (*Nullifier*).
   - Bloqueo atómico de intentos de doble voto mediante consultas determinísticas al *World State* antes de persistir la transacción.

2. **Privacidad Absoluta con Criptografía Homomórfica (Paillier)**:
   - Los votos emitidos se almacenan en texto cifrado (*Ciphertext*).
   - Permite la agregación directa de votos cifrados en el tally sin necesidad de descifrar ningún voto individual.

3. **Pruebas de Conocimiento Cero (ZKP)**:
   - Cada voto incluye una prueba ZKP asociada para validar su pertenencia a un rango legítimo sin desvelar la opción elegida.

4. **Agregación Homomórfica Eficiente (`QueryTally`)**:
   - Provee un endpoint que realiza un barrido léxico por rango (`GetStateByRange`) para extraer de forma masiva únicamente los textos cifrados, listos para ser agregados por el módulo off-chain **QoriCore**.

5. **Compatibilidad Estricta con Hyperledger Fabric v2.5**:
   - Desarrollado usando `github.com/hyperledger/fabric-contract-api-go/contractapi`.
   - Uso de marcas de tiempo determinísticas (`ctx.GetStub().GetTxTimestamp()`) para evitar desacuerdos entre nodos validadores (endorsers).

---

## 📁 Estructura del Repositorio

```text
.
├── LICENSE                      # Licencia Apache 2.0
├── README.md                    # Documentación general del proyecto
├── EXPLANATION.md               # Análisis técnico detallado de la arquitectura y código
├── build_qorichaincode.sh       # Script unificado de generación y compilación con Docker
├── .gitignore                   # Exclusiones de Git para binarios, logs y artefactos
└── qorichaincode/               # Código fuente del Chaincode Go
    ├── Dockerfile.build         # Entorno efímero de validación y compilación (Alpine + Go 1.20)
    ├── main.go                  # Punto de entrada y arranque del chaincode
    └── smartcontract.go         # Lógica de negocio, validación de estado y consultas
```

---

## ⚙️ Especificación de la API del Smart Contract

### 1. `CastVote(ctx, nullifier, ciphertext, zkp)`
Registra un nuevo voto en el ledger.

* **Parámetros**:
  * `nullifier` *(string)*: Hash/identificador único e irreversible del votante derivado de su credencial criptográfica.
  * `ciphertext` *(string)*: Representación en cadena del voto cifrado con la clave pública de Paillier de la elección.
  * `zkp` *(string)*: Prueba criptográfica de conocimiento cero que certifica la validez del voto cifrado.
* **Clave de Estado Generada**: `VOTE_<nullifier>`
* **Validación**: Si la clave ya existe en el estado del ledger, aborta la transacción arrojando:
  `"Doble voto detectado: el nullifier ya ha sido utilizado"`.
* **Retorno**: `nil` en caso de éxito, o `error` si la validación falla.

---

### 2. `QueryTally(ctx)`
Recupera todos los votos emitidos para la fase de escrutinio homomórfico.

* **Operación**: Ejecuta un barrido léxico de claves en el rango `["VOTE_", "VOTE_~"]`.
* **Retorno**: `[]string` conteniendo únicamente los textos cifrados (`Ciphertext`) de todos los votos válidos registrados.
* **Consumo**: Diseñado para el backend criptográfico **QoriCore**, el cual ejecuta la multiplicación modular homomórfica:
  $$\mathbf{C}_{\text{total}} = \prod_{i=1}^{N} c_i \pmod{n^2}$$

---

## 🚀 Compilación y Validación de Integridad

El repositorio incluye un script unificado que automatiza la generación de archivos y valida la compilación estática dentro de un contenedor Docker efímero:

```bash
chmod +x build_qorichaincode.sh
./build_qorichaincode.sh
```

### ¿Qué hace el script?
1. Escribe o actualiza los archivos fuente dentro de la carpeta `qorichaincode/`.
2. Lanza una compilación en contenedor basado en `golang:1.20-alpine`.
3. Ejecuta `go mod init qorichaincode` y `go mod tidy` descargando el SDK oficial de Fabric.
4. Compila un binario estático con `CGO_ENABLED=0` (`go build -v -ldflags="-s -w" -o chaincode .`).
5. Limpia automáticamente la imagen de construcción tras finalizar exitosamente.

---

## 📦 Despliegue en Red Hyperledger Fabric (Peer Lifecycle)

Para empaquetar y desplegar el chaincode en un canal de Fabric v2.5:

```bash
# 1. Empaquetar el chaincode
cd qorichaincode
peer lifecycle chaincode package qorichaincode.tar.gz \
  --path . \
  --lang golang \
  --label qorichaincode_1.0

# 2. Instalar en el peer
peer lifecycle chaincode install qorichaincode.tar.gz

# 3. Aprobar para la organización (ejemplo Org1)
peer lifecycle chaincode approveformyorg -o localhost:7050 \
  --channelID mychannel \
  --name qorichaincode \
  --version 1.0 \
  --package-id <PACKAGE_ID> \
  --sequence 1 \
  --tls --cafile <ORDERER_CA>

# 4. Comprometer en el canal
peer lifecycle chaincode commit -o localhost:7050 \
  --channelID mychannel \
  --name qorichaincode \
  --version 1.0 \
  --sequence 1 \
  --tls --cafile <ORDERER_CA>
```

---

## 📄 Licencia

Este proyecto está bajo la Licencia [Apache 2.0](LICENSE).
