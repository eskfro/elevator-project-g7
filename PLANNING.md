Planning for the Elevator Project

### MODULES AND VARIABLES

Requests (info about the local elevator)
- LocalOrderTable

NetworkModule (provided network module)

Elevator_IO (interacts with driver)
- ElevatorStates (floor, direction, etc.)

ElevatorMovement (fsm)

OrderController (tables for the elevator)
- ActiveOrderTable
- PotentialOrderTable
- UnassignedOrderList
- ClearList

RecieveOrders (tables for backup things)(we may drop this)

ConnectedElevators (p2p alive messages)
- AliveList

## W5

### Communications

### OrderEvent
format = (epoch, N, A)

#### M broadcasting
- i am alive (state)
#### B broadcasting
- i am alive (state)

#### Messages from B to M (new order)
- new order:
	(")


### Questions 
- Burde vi snakke om den utdelte heiskoden?

### List of figures to make:
-> Hvordan sørge for at det er trygt å skru på ordrelys (hovedide backup er klart). Master trenger ack fra heisen som skal ta ordre
-> Network: idempotency and commutativity (ordning), P2P I am alive
      korrupt: sjekk om meldingene er det samme
-> Spontaneous crashes: Backup løsningen og at lys ikke skrus på før
      en backup er klar og fortsatt vil huske ordren. Backup blir til master diagram. En annen case, er backup crasher
-> unscheduled restarts: Dette er samme løsning som med crash, men vi burde spesifisere hvordan en backup reentrer.
-> Normal operation of hall call. Det blir sekvensdiagrammet vi drev.
-> Normal operation of cab call. Det blir sekvensdiagrammet vi drev.
-> detection og takeover. Hvordan hall requests redistribueres
-> Unngår Split Brain med master epoch
-> Network topology, master slave and P2P       (lag mot slutten)
-> Moduldiagram                     (lag mot slutten)


### case: Init Backup
- Start program
- Broadcast "I am alive"

### Button light contract
- Solution:
	- Oppdaterer backups på ny ordre før vi skrur på lyset før ordren er offisiell
- Sekvens:
	- Ny ordre kommer inn
	- Ny ordre deles til master
	- Master regner hvilken heis
	- Master sender ut ordre til backups
	- Backuåps skru på sitt lys når de får ordren
	- Master skrur på lys når den som tar ordren acker
- Hvorfor er dette trygt ? 
	- Når første backup har fått ordren har to enheter ordren som betyr at backup fungerer
	- Master venter fordi den er klar over at to vet om det
	- Om master ikke har fått ack fra heis som skal ta ordren vil ordren redistribueres
	- Dersom ny master skal velges må alle backup sende sin nyeste state, da er det nok at bare en vet om dette for at staten bevares. 

### Network Unreliability
Må ta hensyn til:
- Packet loss
	- 
- Loss of connection
	- Ha en del av systemet der alle kommunisere "I am alive" med p2p
	- Brukes for å holde oversikt over hvilke enheter som har kontakt med hverandre og vil brukes av master på hvilke heiser som vil motta hvilke ordre. 
- Rekkefølge på meldinger
	- Idempotente meldinger som angis et ordensnummer slik at rekkefølgen blir uproblematisk. Med idompotente meldinger menes med at master sender på hver ordre all informasjon som må vites som inkluderer tidligere endringer. Oversikt over hall og cab ordrer. 
	- Hver nye ordre får en oppdatert nummer som da indikerer nyeeste ordre / state. 
	- Master broadcaster nye ordrer slik at dersom noen ikke mottar denne vil de motta neste.

### Spontanious Crashes
- Master crasher
	- Håndteres ved at backup blir til master (LAG DIAGRAM)(X)
	- Se diagram Casper sendte, lag denne i Draw.io
- Backup crasher
	- Backup sine hall orders redistribueres og ingen nye ordre til denne
	- Når backup kommer tilbake får den tilsent sine cab orders 
	- OrderControl burde deskrive denne situasjonen 
	- Sekvensdiagram for ny Backup på nettverk (LAG DIAGRAM)(X)
		- id = ip adresse
	
### Detection og Takeover
case: Oppdages av slavekeeper at backup er død (LAG DIAGRAM)
- Heisens hall orders redistribueres, se OrderControl diagram.
case: Ordren gjennomføres ikke innen tidsfrist
- Det blir oppdagt og en ny heis blir satt til å ta ordren. 

### Master split brain
- Dette kommer frem i diagrammet hvord det beskrives hvordan nye master velges
- Kort sagt handler løsningen om epoch numer og hvordan master velges 
- Det vil here tiden være mulig å detektere en falsk master


### Ideas (ESKIL) 26.01.26

Ha med i Rapport:
- Network protocol
	- UDP broadcasting
	- Master slave for order distr
	- p2p2 for alive entity list, to know who is on the network
- Programming language
	- Message passing for concurrent threads
- Diagrammer:
	- Backup blir til master
	- Backup crasher
- Unscheduled restart
- Redistribution of hall requests when one backup dies
	- 


Port logikk
- port_MB //Master state broadcasting
	- Master sender hele staten med nummerering
	- melding = (epoch, S_num, data)
	- Backupene tar imot meldinger på samme port
- port_BM //Backup broadcasting
	- Master listens på denne kanalen
	- Backups repiterer nye beskjeder fra master
- case BACKUP -> MASTER
	- begynne å sende på port_MB og motta på port_BM	
- case MASTER -> BACKUP ??? // unødvendig
	- Dersom alle backup ikke enig i hvem som skal være master
	- bare start programmet på nytt elns



## W4

### Plan  (W4)

TODO
- bestemme port logikk ved "i am alive" melding

p2p aspect
- hvem lever
- brukes ved slave -> master
- eskil: master sender melding og slaver acker. Dersom slavene ikke får meldinger fra master så må alle sammen anta at master er død, da blir alle enige om hvem som er neste ved at alle har samme logikken.

master-backup com
- master sender hele tilstanden
- nummerer state oppdateringer
- master sender samme informasjon til alle backups
- MB broadcasting com skjer på en port (port_MB)
- eskil: etter det må vel alle slavene 
- master repiterer nyeste beskjeden sin periodisk
- epoch: master nummer
- S_num: nummer på state
- melding: (epoch, S_num, data)
- data: HallOrderTable, CabOrderTable

Ny hall order case
1. Knapp trykkes på en  backup
2. Backup videresender event til ordreport
3. Master oppfatter ny ordre og kalkulerer hvem som skal ta den
4. Master legger ny ordre i sin PotentialState table
5. Master sender PotentialState til backups
6. Backups lagrer mottatt beskjed som sin ActiveState. Oppdaterer epoch og S_num.
7. Heisen som fikk ordre acker på ordre recieve porten tilbake til master
8. Når heisen får relevant ack oppdaterer den sin ActiveState Table. Da går lyset til master på. 

Backup blir master
1. master ikke vist livstegn
2. Hver backup bestemmer ny master med felles regler
3. Nye master broadcaster med sin nye epoch med S_num = 0, dette skjer på master sin port.
4. Backups svarer om de er enige eller ikke. Evt svarer de hvem som faktisk skal være master
5. Dersom master blir motbevist går den til backup
6. Ellers starter den med master oppgaver

How to clear requests
- Problem: Backup must inform master that order is cleared, but master will in the meantime still update states, saying that the order is still active
- Solution 1: The backup who makes the order will not reinitialize its HallOrderTable / CabOrderTable unless (epoch, S_num) > prev (epoch, S_num) indicating a new order actually was called in the floor. 

OrderTable
- Includes hall requests for all elev
- Includes cab request for all elev
- ex: entry (5, 12) = (epoch, S_num) = order
- ex: entry (0, 0) =  no order

OrderControl (master)
while new:
	if new order:
		ComputeWhichElevator()
		update PossibleOrderTable()
		update (epoch, S_num, data) for sending
		wait for acks
		-> update ActiveOrderTable()
		-> start timer for order
	elif clear order:
		Check if order can be cleared
		update ActiveOrderTable
		update (epoch, S_num, data) for sending

Backup dør
- ???

SlaveKeeper modulen
- Nebytt p2p com

### What backup has to know
- Unassigned orders
- HallOrderTable // Hvilke heiser som betjener ordrene
- AliveList ?

### When to update OrderTable
- New order / redistributuon of orders
	- It gets a message from the order assign/recieve verifier
- Order is done for
	- The elevator is in the correct floor
	- The elevator door is open
	- When clear current floor is called:
		- Master should be notified
		- There is a message from request from the distributor from the slave
			- The slave can clear it at once
		- It neads to send to the master throgh the network that it is cleared
		- The master getsa a message from the slave that goes to order distributor
			- Then master can also remove that order from OrderList
		- Another possibility: 
			- The slave can alert the master and the master notifies everyone, including the slave itsetlf
		- Another possibiloty:
			- The master/slave can have two OrderList and and UnasignedOrderList; One list for what only the slaves possibly know about, and one for the "global ish" list. 
			- This may be better incase we get problems when a master/slave shuts down in the middle of the communication process
	- Lights:
		- When we have verified the orders from backup:
			- The lights should be turned on 
		- When we have verified the confirmation:
			- The light are to be turned off
		- So basicly:
			- The lights are what everyone agrees about

### What happens when a slave becomes a master?
- Make a common rule for the new master
	- Lowest IP = new master
- The new assigned master must:
	- elevator.master = true
	- send i am alive to the backups
	- the master needs to know which backups are alive
	- remove earlier master from alivelist

### What is a new node on the network? 
- The new nod needs to be sent all the data, not only the changes
- Idempotant: A new message overwrites the old 
	- We may also do this

### Program Flow (hall call)
1. Slave gets a button press
2. Slave updates its own UnassignedOrders
3. Slaves sends a new message to master through network
4. Master recieves new order and sends it to OrderController
5. Master writes down this order into the masters UnassignedOrders
6. OrderDistributor (master) forwards this message throught network to slaves
7. All slaves updates their UnassignedOrders
8. Slaves turn on their lamp
9. Slaves ack they have recieved this list
10. Master now knows that the order is distributed and verifies the order
11. Master updates its UnassignedOrders
11. Master turns on its lights
12. OrderController (master) calculates the elevator to take the order (slave2)
13. Master updates: HallOrderTable, UnassignedOrders
14. Master sends a network message to everyone that slave2 takes the order
15. The slaves updates their HallOrderTable
16. The relevant slave (slave2) updates its LocalOrderTable ( e.requests(floor)(ButtonType) )
17. The slaves ack they recieved the order update // ??
18. // If the relevant elevator does not ack the recieved order, the order will re-enter the order distributor
19. The relevant slave (slave2) moves to the floor based on the fsm logic
20. // If this is interrupted the master notices and redistributes the order
21. When the elevator arrives at relevant floor and opens the door the order has been served
22. Then the request module notifies the order module (slave side) that the order is cleared
23. The slave removes this order from its LocalOrderTable (requests)
24. The slave (slave2) notifies the master that the order is cleared
25. The master updates its HallOrderTable and notitifies all the slaves 
26. Resend until slaves ack

#### Comments on program flow (hall call)
- Usikker på om vi faktisk trenger ekstra kopier av listene som masteer ikke kanskje vet
- Dobbeltsjekka at alle disse acksene er nødvending
- Bestem oss for navn på lister det ble litt rot

### Questions
- What is meant by a new node on the network? 

### TODO
- Lage flow for nettverk, cab calls, ny person på nettverks osv
- Lage sekvensdiagram 
- Tilstandsdiagrammer innad i de forskjellige trådene