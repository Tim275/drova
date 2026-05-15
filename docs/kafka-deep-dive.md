# Kafka — Deep Dive (Drova Reference)

---

## 1. Das Problem vor Kafka

### Synchrone Microservice-Kommunikation

Stell dir Drova ohne Kafka vor. Jeder Service redet direkt mit jedem anderen:

```
Rider POST /trip/start
  → trip-service
      → HTTP → driver-service.FindDrivers()     ← BLOCKIERT (wartet)
          → HTTP → driver-service.Notify()       ← BLOCKIERT
              → HTTP → payment-service.Init()    ← BLOCKIERT
                  → HTTP → api-gateway.Push()    ← BLOCKIERT
```

**Konkrete Probleme:**

| Problem | Was passiert |
|---|---|
| driver-service crasht | trip-service kriegt Timeout → Rider sieht 500 |
| 1000 Rider gleichzeitig | alle 1000 Requests stehen in Kette, blockieren sich |
| Neuer analytics-service | trip-service muss angepasst werden um ihn zu informieren |
| trip-service crasht nach DB-Write | Payment wurde nie gestartet, Daten inkonsistent |
| driver-service braucht 2s | Rider wartet 2s auf Antwort obwohl trip-service fertig ist |

Das nennt man **tight coupling** — Services sind direkt voneinander abhängig.
Fällt einer aus, fallen alle aus.

### Synchron vs. Asynchron

**Synchron** (Telefongespräch — beide müssen gleichzeitig verfügbar sein):
```
trip-service:    "Hey driver-service, hier ist Trip #42!"
                 ←←← WARTET ←←← (blockiert, tut nichts)
driver-service:  "OK, hab ihn, suche Fahrer..."
trip-service:    "Gut, hier ist die Antwort für den Rider"
```

**Asynchron** (WhatsApp — Sender macht weiter, Empfänger liest wann er kann):
```
trip-service:    schreibt Event in Kafka → fertig, macht sofort weiter
                              ↓
driver-service:  liest Event 50ms später (oder nach Neustart) → verarbeitet
payment-service: liest DASSELBE Event → verarbeitet unabhängig
api-gateway:     liest DASSELBE Event → pusht an Browser
```

---

## 2. Was Kafka ist

Kafka ist ein **verteiltes, persistentes Event-Log**.

Keine Queue die geleert wird — ein **Logbuch** das Einträge dauerhaft speichert.

```
┌─────────────────────────────────────────────────────┐
│                  Kafka Cluster                       │
│                                                      │
│  Topic: "trip.event.created"                         │
│  ┌────┬────┬────┬────┬────┬────┐                     │
│  │ #0 │ #1 │ #2 │ #3 │ #4 │ #5 │ ...  (Offset)      │
│  └────┴────┴────┴────┴────┴────┘                     │
│                                                      │
│  driver-service liest: Offset 3 ✓                    │
│  api-gateway liest:    Offset 5 ✓                    │
│  analytics liest:      Offset 1 ✓ (noch nachholend)  │
└─────────────────────────────────────────────────────┘
```

Jeder Consumer merkt sich selbst wo er ist (Offset). Kafka löscht nichts — außer nach konfigurierter Retention.

---

## 3. Kern-Konzepte

### Topics

Ein Topic = ein Thema / eine Kategorie von Events. Wie ein Ordner.

**Drova's 14 Topics:**
```
trip.event.created           → Rider hat Trip gebucht
trip.event.driver_assigned   → Fahrer wurde zugewiesen
trip.event.driver_arrived    → Fahrer ist da
trip.event.in_progress       → Fahrt läuft
trip.event.completed         → Fahrt abgeschlossen
trip.event.cancelled         → Fahrt abgebrochen
trip.event.no_drivers_found  → kein Fahrer verfügbar
trip.event.driver_not_interested → Fahrer hat abgelehnt

driver.cmd.trip_request      → Frage an Fahrer: willst du?
driver.cmd.trip_response     → Antwort des Fahrers

payment.cmd.create_session   → Zahlung starten
payment.event.session_created → Stripe URL ist bereit
payment.event.success        → Zahlung erfolgreich

dead.letter.queue            → fehlgeschlagene Messages
```

**Naming-Convention in Drova:**
- `*.event.*` = etwas ist passiert (unveränderlicher Fakt)
- `*.cmd.*` = jemand soll etwas tun (Befehl)

### Partitionen

Jedes Topic ist in Partitionen aufgeteilt — für Parallelismus:

```
Topic: "trip.event.created"   (3 Partitionen)

Partition 0: [Trip#1] [Trip#4] [Trip#7] ...
Partition 1: [Trip#2] [Trip#5] [Trip#8] ...
Partition 2: [Trip#3] [Trip#6] [Trip#9] ...
```

- Reihenfolge ist **innerhalb einer Partition** garantiert
- Über Partitionen hinweg: keine Garantie
- Mehr Partitionen = mehr Parallelismus = höherer Durchsatz

**Partition Key:** Welche Partition bekommt die Message? Über Key.
In Drova: `tripID` als Key → alle Events eines Trips landen in derselben Partition → Reihenfolge pro Trip garantiert.

### Offsets

Jede Message in einer Partition hat eine eindeutige Nummer (Offset):

```
Partition 0: [#0: Trip#1] [#1: Trip#4] [#2: Trip#7]
                                          ↑
                              driver-service ist hier (Offset 2)
```

Consumer merken sich ihren Offset — so wissen sie nach einem Neustart wo sie weitermachen.

### Consumer Groups

```
Topic: "trip.event.created"

Consumer Group "driver-svc":          Consumer Group "gateway-notify":
┌─────────────────────┐               ┌─────────────────────┐
│ driver-service Pod1 │ ← Partition 0 │ api-gateway Pod1    │ ← Partition 0
│ driver-service Pod2 │ ← Partition 1 │ api-gateway Pod2    │ ← Partition 1
│ driver-service Pod3 │ ← Partition 2 │ api-gateway Pod3    │ ← Partition 2
└─────────────────────┘               └─────────────────────┘
  → jedes Event EINMAL                  → jedes Event EINMAL
    (Load Balancing)                      (eigene Kopie)
```

**Gleiche Group ID** = Event geht an genau EINEN Consumer der Gruppe → Horizontal skalierbar
**Verschiedene Group ID** = jede Gruppe bekommt alle Events → Pub/Sub

In Drova:
```go
// driver-service + api-gateway lesen trip.event.created
// ABER mit verschiedenen Group IDs → beide bekommen jeden Event
kafka.ConsumeMessages(ctx, TopicTripCreated, "driver-service-group", handler)
kafka.ConsumeMessages(ctx, TopicTripCreated, "gateway-notify-group", handler)
```

### Producer

Schreibt Events in ein Topic:
```go
// trip-service nach CreateTrip():
kafkaClient.Publish(ctx, TopicTripCreated, KafkaMessage{
    Type:    "trip.event.created",
    OwnerID: userID,        // wer bekommt die WS-Notification
    Data:    json.Marshal(trip),
})
// → fertig, macht sofort weiter (async!)
```

### Consumer

Liest Events aus einem Topic:
```go
// driver-service:
kafka.ConsumeMessages(ctx, TopicTripCreated, "driver-svc", func(msg []byte) error {
    var event TripCreatedEvent
    json.Unmarshal(msg, &event)
    // Fahrer suchen, trip_request publishen
    return nil
})
```

### Manual Acknowledgement (Drova nutzt das)

```go
// shared/messaging/kafka.go
msg := reader.FetchMessage(ctx)     // holen — noch NICHT bestätigt

err := handler(msg.Value)           // verarbeiten
if err != nil {
    log.Error("handler failed")
    continue                        // NICHT committen
    // Kafka liefert dieselbe Message nochmal
}

reader.CommitMessages(ctx, msg)     // JETZT bestätigen
```

**Warum wichtig:** driver-service crasht mitten beim Verarbeiten → Message nicht committed → beim Neustart wieder geliefert → kein Trip geht verloren.

---

## 4. Der vollständige Drova Trip-Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         KAFKA EVENT BACKBONE                            │
│                                                                         │
│  ① Rider POST /trip/start                                               │
│     trip-service ──[trip.event.created]──────────────────────────────► │
│                                                                         │
│  ② driver-service konsumiert                                           │
│     ◄──[trip.event.created]── driver-service                           │
│     driver-service ──[driver.cmd.trip_request]───────────────────────► │
│                                                                         │
│  ③ api-gateway pusht an Driver-Browser                                 │
│     ◄──[driver.cmd.trip_request]── api-gateway                         │
│     api-gateway ──WebSocket──► Driver: "Neue Fahrtanfrage!"            │
│                                                                         │
│  ④ Fahrer akzeptiert (WS → api-gateway)                                │
│     api-gateway ──[driver.cmd.trip_response]─────────────────────────► │
│     trip-service konsumiert → UpdateStatus("accepted")                 │
│     trip-service ──[trip.event.driver_assigned]──────────────────────► │
│     api-gateway konsumiert → WebSocket → Rider: "Fahrer kommt!"       │
│                                                                         │
│  ⑤-⑥ arrived + in_progress (analog)                                    │
│                                                                         │
│  ⑦ Fahrt endet: driver.cmd.trip_end                                    │
│     trip-service ──[payment.cmd.create_session]──────────────────────► │
│                                                                         │
│  ⑧ payment-service erstellt Stripe Session                             │
│     ◄──[payment.cmd.create_session]── payment-service                  │
│     payment-service ──[payment.event.session_created]────────────────► │
│     api-gateway ──WebSocket──► Rider: Stripe-URL                       │
│                                                                         │
│  ⑨ Stripe Webhook → payment.event.success                              │
└─────────────────────────────────────────────────────────────────────────┘
```

**Was Kafka hier konkret löst:**

| Szenario | Ohne Kafka | Mit Kafka |
|---|---|---|
| driver-service kurz down | Trip verloren | Event bleibt, wird nach Restart verarbeitet |
| 500 Trips gleichzeitig | Services blockieren sich | Kafka puffert, jeder Service verarbeitet in eigenem Tempo |
| Neuer analytics-service | trip-service muss angepasst werden | Liest einfach trip.event.created mit eigener Group ID |
| payment-service crasht nach Stripe-Call | Doppelzahlung möglich | Idempotency-Key + Manual Ack verhindert das |

---

## 5. KRaft — Kafka ohne ZooKeeper

**Alt (bis Kafka 2.7):**
```
┌─────────────────────────────────────┐
│ ZooKeeper Cluster (3 Nodes)         │  ← separates System
│ koordiniert Kafka-Cluster-State     │
└───────────────────┬─────────────────┘
                    │
┌───────────────────▼─────────────────┐
│ Kafka Cluster (3 Broker)            │
└─────────────────────────────────────┘
```

**Neu (KRaft, ab Kafka 2.8, Drova nutzt das):**
```
┌─────────────────────────────────────┐
│ Kafka Cluster (3 Nodes)             │
│ ├── Node 1: broker + controller     │  ← alles in einem
│ ├── Node 2: broker + controller     │
│ └── Node 3: broker + controller     │
└─────────────────────────────────────┘
```

Drova docker-compose:
```yaml
KAFKA_PROCESS_ROLES: broker,controller
KAFKA_CONTROLLER_QUORUM_VOTERS: 1@127.0.0.1:9093
CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk
```

**Vorteile KRaft:**
- Kein separates ZooKeeper zu managen
- Schnellerer Leader-Election
- Einfacheres Deployment (ein System statt zwei)
- Kubernetes-freundlicher (ein StatefulSet statt zwei)

---

## 6. Schema Registry

### Das Problem ohne Schema Registry

```
trip-service v1 publishes:
{ "tripID": "abc", "userID": "123", "status": "searching" }

trip-service v2 publishes (Feld umbenannt):
{ "trip_id": "abc", "user_id": "123", "state": "searching" }

driver-service (noch v1-Code):
→ json.Unmarshal → tripID ist leer → Bug
```

Ohne Schema Registry: Producer und Consumer müssen manuell koordiniert werden. In Microservices-Umgebungen mit unabhängigen Deploy-Zyklen: unmöglich.

### Schema Registry Architektur

```
┌──────────────────────────────────────────────────────────────────┐
│                      Schema Registry                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ Subject: trip.event.created                              │    │
│  │  v1: { tripID: string, userID: string, ... }            │    │
│  │  v2: { tripID: string, userID: string, fare: object }   │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
          ▲ Register/Validate              ▲ Fetch Schema
          │                               │
    ┌─────┴──────┐                 ┌──────┴──────┐
    │  Producer  │                 │  Consumer   │
    │ trip-svc   │──── Kafka ─────►│ driver-svc  │
    └────────────┘                 └─────────────┘
```

**Ablauf:**
1. Producer registriert Schema in Registry (einmalig)
2. Producer serialisiert Message mit Schema-ID im Header
3. Consumer holt Schema-ID aus Header → lädt Schema → deserialisiert

### Avro vs. Protobuf vs. JSON Schema

| Format | Kompression | Schema Evolution | Drova |
|---|---|---|---|
| JSON | keine | manuell | aktuell (Dev) |
| Avro | gut | automatisch (Compatibility) | geplant |
| Protobuf | sehr gut | automatisch | intern für gRPC |

Drova hat `shared/schema/` bereits vorbereitet (hamba/avro) — noch nicht aktiv verdrahtet.

### Compatibility Modes

Schema Registry prüft ob neue Schema-Version rückwärts-kompatibel ist:

```
BACKWARD:  neue Consumer können alte Messages lesen  (sicherster Default)
FORWARD:   alte Consumer können neue Messages lesen
FULL:      beides
NONE:      keine Prüfung (gefährlich)
```

**Erlaubte Änderungen (BACKWARD):**
- ✅ Neues optionales Feld hinzufügen
- ✅ Feld entfernen das einen Default hat
- ❌ Pflichtfeld umbenennen
- ❌ Typ ändern (string → int)

### Drova Schema Design

```go
// shared/schema/schemas.go — Avro Schema für TripCreated
const TripCreatedSchema = `{
  "type": "record",
  "name": "TripCreated",
  "namespace": "drova.trip",
  "fields": [
    {"name": "trip_id",   "type": "string"},
    {"name": "user_id",   "type": "string"},
    {"name": "status",    "type": "string"},
    {"name": "fare_id",   "type": ["null", "string"], "default": null},
    {"name": "created_at","type": "long"}
  ]
}`
```

Neues Feld `fare_id` als nullable mit default → BACKWARD compatible → alte Consumer ignorieren es.

---

## 7. Das Kafka-Ökosystem

### Kafka Connect

Daten automatisch zwischen Kafka und externen Systemen synchronisieren — ohne Code:

```
PostgreSQL ──[CDC Debezium Connector]──► Kafka ──[S3 Sink]──► S3
              (jede DB-Änderung als Event)
```

**Für Drova relevant wenn:**
- Location-History aus Redis/Kafka → ClickHouse (Analytics)
- Trips-Daten → Data Lake (S3 Iceberg für ML)
- Stripe Events → Data Warehouse

**Connector-Typen:**
- **Source Connector:** externes System → Kafka (z.B. Debezium für PostgreSQL CDC)
- **Sink Connector:** Kafka → externes System (z.B. S3, Elasticsearch, BigQuery)

### Kafka Streams / ksqlDB

Stream Processing direkt auf Kafka Topics — SQL-ähnlich:

```sql
-- Surge Pricing für Drova: Demand-Ratio live
CREATE TABLE surge_factor AS
SELECT
  city,
  COUNT(*) FILTER (WHERE status = 'searching') AS demand,
  COUNT(*) FILTER (WHERE status = 'available') AS supply,
  CASE
    WHEN supply = 0 THEN 3.0
    WHEN demand / supply > 2 THEN 1.5
    ELSE 1.0
  END AS factor
FROM trips_stream
WINDOW TUMBLING (SIZE 5 MINUTES)
GROUP BY city;
```

→ payment-service liest `surge_factor` → multipliziert Preis

**Operationen:**
- `filter` → Messages filtern
- `map` → Messages transformieren
- `join` → Topics joinen (z.B. trip + driver → enriched trip)
- `aggregate` → Windowed Aggregation (5-Minuten-Fenster)
- `groupBy` → nach Key gruppieren

### MirrorMaker 2

Kafka-Cluster zwischen Rechenzentren replizieren:

```
EU-West Cluster                    EU-East Cluster
┌─────────────────┐                ┌─────────────────┐
│ trip.events     │──MirrorMaker──►│ eu-west.trip.   │
│ driver.events   │                │   events        │
│ payment.events  │                │ eu-west.driver. │
└─────────────────┘                │   events        │
                                   └─────────────────┘
```

**Use Cases:**
- Disaster Recovery (EU-West down → EU-East übernimmt)
- Geo-Latenz (Fahrer in München → EU-Central, in Wien → EU-East)
- Compliance (DSGVO: EU-Daten bleiben in EU)

**Drova aktuell:** Ein Cluster, kein MirrorMaker nötig. Relevant bei Multi-Region.

### Strimzi (Kafka on Kubernetes)

Kafka Cloud-Native auf Kubernetes betreiben via Operator-Pattern:

```yaml
# Kein manuelles StatefulSet — Strimzi Operator managed alles
apiVersion: kafka.strimzi.io/v1beta2
kind: Kafka
metadata:
  name: drova-kafka
spec:
  kafka:
    replicas: 3
    storage:
      type: persistent-claim
      size: 100Gi
    config:
      default.replication.factor: 3
      min.insync.replicas: 2
  zookeeper:           # oder KRaft mode
    replicas: 3
```

Strimzi managed: Rolling Updates, TLS, SASL Auth, Topic-Operator (KafkaTopic CRD), User-Operator.

**Drova Homelab:** Kafka läuft als einzelner Container (kein Strimzi) — für Prod würde man Strimzi einsetzen.

### Dead Letter Queue (DLQ)

Drova hat `dead.letter.queue` als Topic:

```go
const TopicDeadLetterQueue = "dead.letter.queue"
```

Wenn ein Consumer nach N Retries immer noch scheitert → Message in DLQ → kein endloser Retry-Loop → separate Behandlung (Alert, Manual Review, Replay).

```
Normal Flow:
Message → Consumer → Fehler → Retry 1 → Retry 2 → Retry 3 → DLQ

DLQ Handling:
DLQ → Alert → Entwickler schaut rein → Manual Replay wenn gefixt
```

---

## 8. Kafka Guarantees

### Delivery Semantics

**At-most-once** (Drova nutzt das NICHT):
- Producer sendet, vergisst
- Consumer committet vor Verarbeitung
- Bei Crash: Message verloren

**At-least-once** (Drova nutzt das):
- Producer wartet auf Acknowledgement
- Consumer committet NACH Verarbeitung (Manual Ack)
- Bei Crash: Message wird nochmal geliefert → **Idempotency nötig**

**Exactly-once** (Kafka Transactions):
- Komplexeste Garantie
- Für Drova relevant bei Payment (nie doppelt abbuchen)

### Idempotency in Drova

```go
// payment-service/events/trip_consumer.go
// Verhindert Doppelzahlung bei Kafka-Redelivery:
existing, err := store.GetByTripID(ctx, event.TripID)
if existing != nil {
    return nil  // bereits verarbeitet — ignorieren
}
// → Stripe Session erstellen
```

```go
// api-gateway/http.go — Stripe Webhook Dedup:
set := rdb.SetNX(ctx, "drova:webhook:stripe:"+sessionID, 1, 24*time.Hour)
if !set {
    return  // Stripe retry — schon verarbeitet
}
```

### Replication Factor & ISR

```
Topic: trip.event.created, replication-factor=3

Broker 1 (Leader):  [#0] [#1] [#2] [#3]  ← Producer schreibt hier
Broker 2 (Replica): [#0] [#1] [#2] [#3]  ← In-Sync Replica (ISR)
Broker 3 (Replica): [#0] [#1] [#2]       ← 1 hinter (Syncing)
```

`min.insync.replicas=2`: Mindestens 2 Replicas müssen bestätigen bevor Producer ACK kriegt → kein Datenverlust wenn ein Broker crasht.

---

## 9. Monitoring & Observability

### Key Metrics (Kafka-side)

```
Consumer Group Lag:           wie weit ist Consumer hinter Producer?
  → > 10.000: Consumer zu langsam oder down

Messages per Second:          Throughput
  → Baseline kennen, Alert bei -50%

Under-Replicated Partitions:  Replikation kaputt?
  → > 0: kritisch

Broker Disk Usage:            Retention läuft ab?
  → > 80%: Retention verkleinern oder Disk erweitern
```

### Alert Rule Beispiel

```yaml
- alert: KafkaConsumerLag
  expr: kafka_consumer_group_lag > 10000
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Consumer {{ $labels.group }} lag {{ $value }} messages"
```

### Drova OTEL-Collector → Kafka Metriken

Der OTEL-Collector exposed Kafka-Receiver-Metriken:
```
otelcol_receiver_accepted_spans_total    → wie viele Spans empfangen
otelcol_exporter_sent_spans_total        → wie viele exportiert
otelcol_exporter_send_failed_spans_total → wie viele fehlgeschlagen
```

---

## 10. Lokales Setup (Drova docker-compose)

```yaml
kafka:
  image: confluentinc/cp-kafka:7.6.0
  network_mode: host      # k3d E2E: Pods erreichen Kafka via 172.17.0.1
  environment:
    KAFKA_NODE_ID: 1
    KAFKA_PROCESS_ROLES: broker,controller   # KRaft — kein ZooKeeper
    KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
    KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://127.0.0.1:9092
    KAFKA_CONTROLLER_QUORUM_VOTERS: 1@127.0.0.1:9093
    KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk
```

**Kafka UI:** http://localhost:8080 — Topics, Consumer Groups, Messages live ansehen

**Topics werden beim Start automatisch erstellt:**
```go
// services/api-gateway/main.go
kafka.EnsureTopics(ctx, brokers, []string{
    TopicTripCreated,
    TopicDriverTripRequest,
    // ... alle 14
})
```

---

## 11. Jaeger + OpenTelemetry (Drova Observability Stack)

### Lokaler Stack

```
Go Service
  │ OTel SDK (shared/tracing/)
  │ OTLP HTTP :4318
  ▼
otel-collector
  │ OTLP gRPC → jaeger:4317 (Traces)
  │ Prometheus → prometheus:8889 (Metrics)
  │ OTLP HTTP → loki:3100/otlp (Logs)
  ▼
Jaeger :16686    ← Traces visualisieren
Prometheus :9090  ← Metrics
Loki             ← Logs
Grafana :3000    ← Dashboard für alles
```

### Status

- `user-service` + `api-gateway`: Traces werden gesendet ✅
- `trip-service`, `driver-service`, `payment-service`, `chat-service`: OTel-SDK noch nicht initialisiert ❌

### Jaeger v2 API-Quirk

`/api/services` ohne `?since=` Parameter → gibt hardcoded `"jaeger-all-in-one"` zurück (bekannter Bug).
`/api/services?since=<timestamp>` → gibt echte Services zurück ✅
Grafana UI sendet immer mit Timerange → funktioniert korrekt.

### OTel initialisieren (für die fehlenden Services)

```go
// shared/tracing/tracing.go — bereits vorhanden
func Init(serviceName string) func() {
    exporter, _ := otlptracehttp.New(ctx,
        otlptracehttp.WithEndpoint(os.Getenv("OTEL_COLLECTOR_ENDPOINT")),
        otlptracehttp.WithInsecure(),
    )
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
        )),
    )
    otel.SetTracerProvider(tp)
    return func() { tp.Shutdown(ctx) }
}

// In main.go der fehlenden Services:
shutdown := tracing.Init("trip-service")
defer shutdown()
```

---

## 12. Quick Reference — Kafka-Befehle

```bash
# Topics auflisten
docker compose exec kafka kafka-topics \
  --bootstrap-server localhost:9092 --list

# Consumer Group Lag prüfen
docker compose exec kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --describe --group driver-service-group

# Messages in Topic lesen (von Anfang)
docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic trip.event.created \
  --from-beginning

# Topic erstellen
docker compose exec kafka kafka-topics \
  --bootstrap-server localhost:9092 \
  --create --topic my.topic \
  --partitions 3 --replication-factor 1
```

---

## 13. Zusammenfassung — Was Kafka für Drova leistet

```
VORHER (ohne Kafka):
trip-service → HTTP → driver-service → HTTP → payment-service
     ↑ fällt einer aus, fällt alles aus
     ↑ jeder weiß von jedem
     ↑ keine Retry-Garantie

NACHHER (mit Kafka):
trip-service   }
driver-service } → sprechen NUR mit Kafka
payment-service}   kennen sich nicht gegenseitig

Kafka garantiert:
  ✅ At-least-once Delivery (Manual Ack)
  ✅ Horizontale Skalierung (Consumer Groups)
  ✅ Event-Sourcing (Replay möglich)
  ✅ Entkopplung (Services sind unabhängig deploybar)
  ✅ ML-Readiness (alle Events persistent → Data Lake)
  ✅ Erweiterbarkeit (neuer Service liest einfach mit)
```

**Kafka = wichtigste Architektur-Entscheidung in Drova.**
Jedes Event durch Kafka = potenzielles ML-Trainingsmaterial.
Einfach nach S3 sinken wenn Datenmenge akkumuliert ist.
