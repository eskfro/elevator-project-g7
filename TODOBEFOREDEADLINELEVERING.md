- 1 Alle heisene må lyse når det er en confirmed hall ordre i (floor, btn) //DONE (Tok masse lengre tid enn vi trudde)
- 2 Fjern ch_ // DONE
- 3 Heile oppsettet // DONE
- 4 Fikse <-chan eller chan<- som parameter i ALLE funksjoner //DONE

- 6 Fjerne funksjoner vi ikke bruker // DONE (ish)
- 7 Ordne snakeCase: AliveList -> aliveList, utenom i structs der vi vil det skal være public //Har ikkje sett på medlemsvariable i structs, ellers done
- 8 Når en ny primary velges blir det noen ganger en stuck ordre som står confirmed og ikke vil cleares. Sikkert noe med clearingen å gjøre //DONE

NB! Husk å aldri sende over channels unødvendig, fordi man bruker channels som events, hvis ikke blir alt fort fucked!!! 

TING FØR FREDAG:
- LAB: Teste systemet på LAB der vi plugger ut netverkskabel
- LAB: Teste om en primary blir til backup når nettverkskabelen plugges inn igjen
- LAB: Packetloss under normal operasjon
- Sjekk golangci-lint (Casper anbefalte)