# Utrata potwierdzonego zapisu pod `bridge-churn`: dochodzenie i poprawka

Data: 2026-10-10.
- Gałąź: `fix/follower-ack-durability`.
- Kod przed poprawką: commit `9667921`.
- Dowody: `results/lost-write-2026-10-10/`.
- Oryginalna nieudana próba jest nietknięta w `results/faults-2026-10-10/correct/bridge-churn/trial-001/`.

Oznaczenia:
- **[zweryfikowane]**: wynika z logów, historii albo testu.
- **[wniosek]**: rozumowanie, którego nie sprawdzono bezpośrednio.

## 1. Zaobserwowane naruszenie

Uruchomienie: build bez wstrzykniętych błędów, `rkharness -clients 25 -trials 3 -unique-values -key-rotate 15s -time 30s -fault bridge-churn`, bez `-lazyfs`.

- **Utracony zapis** [zweryfikowane, `history.jsonl`]:
  - `write(r1, 4581)` od procesu 6 (węzeł domowy n2).
  - Wywołany o 7.575 s, potwierdzony (`ok`) o 7.649 s.
- **Odczyty po utracie** [zweryfikowane]:
  - Odczyty `r1` wywołane o 9.63–9.72 s, zakończone o 10.33 s przez nowego lidera, zwróciły 4199, czyli wartość zapisaną wcześniej (7.51–7.56 s).
  - `analysis/stale_reads.py` oznacza je jako na pewno nielinearyzowalne.
  - Porcupine zwrócił `Unknown` (limit czasu), więc to skan, a nie checker, dowodzi naruszenia.
- **Indeks utraconego wpisu** [zweryfikowane]:
  - Przed wywołaniem tego zapisu zakończyło się 4481 potwierdzonych operacji write/CAS. Każda zajmuje osobny wpis, a do tego dochodzi no-op lidera z kadencji 1.
  - Utracony wpis ma więc indeks ≥ 4483.

## 2. Rekonstrukcja zdarzeń (n1 = węzeł-most, restartowany co 100–400 ms)

| Czas | Zdarzenie | Źródło |
|---|---|---|
| 3.07 s | Partycja `[4 5] <-1-> [2 3]`. n2 jest liderem kadencji 1; n4 i n5 widzą tylko n1. | `nemesis.jsonl` |
| 7.552 s | n1 startuje z `last_index` 4464 | `n1.log` |
| 7.649 s | n2 potwierdza zapis 4581 (indeks ≥ 4483) | `history.jsonl` |
| 7.712 s | n1 startuje ponownie z `last_index` **4482** | `n1.log` |
| 7.963 s | n1 startuje ponownie z `last_index` **4482** | `n1.log` |
| 8.155 s | Koniec partycji | `nemesis.jsonl` |
| 8.805 s | n2 kandyduje z `last_index` 4555 | `n2.log` |
| 8.11 / 8.26 s | n4 i n5 kandydują z `last_index` 2140 / 2154 | `n4.log`, `n5.log` |
| 10.363 s | n1 zostaje liderem kadencji 9 (`last_index` 4483 = 4482 + no-op), z głosami n4 i n5 | `n1.log` |
| 10.364 s | n2 i n3 obcinają log od indeksu 4483 | `n2.log`, `n3.log` |

**Kto miał wpis w chwili potwierdzenia.**
- Zatwierdzenie o 7.649 s wymagało 3 z 5 węzłów.
  - n4 i n5 miały ≤ 2154 wpisów [zweryfikowane], więc większość to musiały być n2, n3 i n1.
  - n2 i n3 miały wpis: n2 miał 4555, a n3 został obcięty od 4483 [zweryfikowane].
- **n1 musiał więc zgłosić `MatchIndex` ≥ 4483.** Mimo to żaden późniejszy start n1 nie miał więcej niż 4482 wpisów [zweryfikowane].
- Bez `-lazyfs` dane przekazane do `write(2)` przetrwają `kill -9`. Zatem n1 potwierdził wpisy, których jeszcze nie przekazał do pamięci trwałej [wniosek z powyższych faktów, potwierdzony w §3].
- Wykluczyłem inne miejsca, w których n1 mógłby potwierdzić wpis bez pamięci trwałej:
  - Ścieżki snapshotu: n1 nie miał snapshotu (`snapshot: 0`) i nie instalował go (0 linii `installing snapshot`).
  - Rozdarty koniec WAL-a: `wal.Load` obcina go po cichu, więc logi tego nie wykluczają. §3 pokazuje jednak inną, wystarczającą przyczynę.

**Elekcja i zatwierdzanie działały zgodnie ze specyfikacją.**
- n1 (kadencja 1, indeks 4482) był „co najmniej tak aktualny” jak n4 i n5 (2140/2154), więc słusznie dostał ich głosy (`HandleRequestVote`, `raft.go:749`).
- `maybeCommit` (`raft.go:527`) poprawnie liczy `match` z odpowiedzi.
- Błąd leży w tym, co follower zgłosił jako `MatchIndex`.

## 3. Przyczyna

`HandleAppendEntries` (`internal/raft/raft.go`, przed poprawką linie 872–881) czekał na zapis tylko wtedy, gdy *to* żądanie zmieniło log:

```go
if len(changed) > 0 {
    seq := r.enqueue(persistOp{ents: slices.Clone(changed)})
    ...
    r.waitDurable(seq)
}
resp.Success = true
resp.MatchIndex = newLast
```

**Jak powstaje błędne potwierdzenie:**
1. Żądanie A dopisuje wpisy do `r.entries` w pamięci, kolejkuje zapis i czeka na niego, zwalniając `r.mu` w `waitDurable`.
2. W tym czasie przychodzi żądanie B, które niczego nie zmienia:
   - retransmisja tych samych wpisów, bo lider po przekroczeniu `RPCTimeout` cofa `next` do `match+1`;
   - albo heartbeat z `PrevIndex` wskazującym za nie, bo przy pipeliningu `next` jest przesuwane z góry.
3. Dla B `changed` jest puste, więc follower od razu odpowiada `Success` z `MatchIndex = newLast`, choć wpisy są tylko w pamięci i w kolejce persystera.
4. Lider liczy to potwierdzenie do większości i zatwierdza wpis.
5. Gdy follower ginie, zanim persister zapisze kolejkę, wpis znika z węzła, który współtworzył większość.

**To narusza wymóg z Figure 2** („respond to RPCs only after updating stable storage”). W konsekwencji łamie Leader Completeness i State Machine Safety: zatwierdzony wpis może nie trafić do logu przyszłego lidera.

**Dowody:**
- **Test jednostkowy** `TestFollowerAckRequiresStableStorage` [zweryfikowane]: na kodzie przed poprawką retransmisja zwraca „match 2, a pamięć trwała kończy się na 0”. Wynik w `results/lost-write-2026-10-10/regression-test-before-fix.txt`.
- **Instrumentacja** [zweryfikowane]:
  - Kopia kodu z `9667921` logowała każdą odpowiedź `Success`, w której `changed` było puste, a `MatchIndex` wykraczał poza zapisane wpisy.
  - Ta sama konfiguracja `bridge-churn` (3 próby) dała 8126, 9320 i 8744 takich odpowiedzi na próbę: 17 247 heartbeatów i 8 943 retransmisji.
  - Przesunięcie `MatchIndex` względem pamięci trwałej: mediana 1 wpis, maksimum 1536.
  - Ścieżka działa więc stale, a utrata wymaga tylko zbiegu z `kill -9`, co `bridge-churn` wymusza na moście.
  - Logi: `results/lost-write-2026-10-10/instrumented-bridge-churn/`.
- **Test regresyjny end-to-end** odtwarza cały scenariusz (§4) i na starym kodzie kończy się dokładnie tą utratą.

To samo zjawisko opisałem wcześniej jako punkt 16.1 w `RAFT-DEEP-DIVE.md`. Tutaj zostało potwierdzone jako przyczyna utraty danych, a nie tylko jako odstępstwo od specyfikacji.

## 4. Testy regresyjne (`internal/raft/durability_test.go`)

- **`TestFollowerAckRequiresStableStorage`** (jeden węzeł, deterministyczny):
  - `gatedStorage` wstrzymuje `Append`, więc wpisy są w pamięci, ale nie w pamięci trwałej.
  - Retransmisja i heartbeat nie mogą zwrócić `Success` z `MatchIndex` większym niż ostatni zapisany indeks.
- **`TestCommittedEntrySurvivesFollowerCrash`** (5 węzłów, sieć w pamięci z `cluster_test.go`):
  - L to lider. F ma wstrzymane zapisy (jak most). G zapisuje normalnie. S1 i S2 są odcięte.
  - Gdy L zatwierdzi X, następuje awaria F przed zapisem, awaria L i G, a potem F wraca razem z S1 i S2.
  - Asercja: nowy lider ma X pod tym samym indeksem.
  - Jeśli L nie zatwierdzi X, bo F go nie potwierdził, test kończy się sukcesem: niezatwierdzony zapis nie podlega gwarancji. Tak zachowuje się kod po poprawce.

**Wyniki:**
- Przed poprawką oba testy padają 30/30, z `-pipeline` i bez niego.
- Po poprawce oba przechodzą 30/30, z `-pipeline` i bez niego.
- Wyjścia: `regression-test-before-fix.txt` i `regression-test-after-fix.txt`.

## 5. Poprawka

`HandleAppendEntries` czeka teraz, aż wszystkie dotychczas zakolejkowane zapisy znajdą się w pamięci trwałej, zanim odpowie `Success`. Dotyczy to także żądań, które niczego nie zmieniły. Jeśli zapis się nie udał (węzeł zatrzymany) albo zmieniła się kadencja, odpowiedź nie jest sukcesem.

**Dlaczego to przywraca niezmiennik:**
- Persister zapisuje operacje w kolejności numerów sekwencyjnych.
- Odpowiedź z `MatchIndex = newLast` wychodzi dopiero po zapisaniu każdej operacji zakolejkowanej przed nią, w tym tej, która wstawiła wpisy do `newLast`.
- Każde potwierdzenie, które lider liczy do większości, odpowiada więc wpisom przetrwającym awarię.
- Czekanie na wszystkie zakolejkowane operacje, a nie na `durableLast`, jest celowe. `durableLast` może być zawyżone po obcięciu logu, więc nie da się na nim polegać.

Interfejsy `Storage` i RPC są bez zmian; zmiana obejmuje 8 linii w jednej funkcji.

## 6. Walidacja po poprawce

| Sprawdzenie | Wynik |
|---|---|
| Nowe testy regresyjne | PASS 30/30 (domyślnie i `-pipeline`) |
| Zestaw testów Raft (`-race`): domyślnie, `-pipeline`, `-batch=1` | PASS we wszystkich trzech |
| `go test -race ./...`, `go vet ./...` | PASS |
| `make bugtests` (4 wstrzyknięte błędy) | Wszystkie 4 nadal wykrywane |
| `bridge-churn`, ta sama konfiguracja, 20 prób | 17 Ok, **3 Unknown**, 0 prób z na pewno nieaktualnymi odczytami |
| E1 (konfiguracja pilotażu, po 3 próby): kill-leader, partition-leader, partition-halves, pause-leader, kill-all + lazyfs | 15/15 Ok, 0 nieaktualnych odczytów |
| E1 ścisłe (`-unique-values -key-rotate 15s`): kill-leader, partition-halves, kill-all + lazyfs | 9/9 Ok, 0 nieaktualnych odczytów |
| ackbeforefsync (kill-all + lazyfs, unikalne wartości) na nowym buildzie | Nadal wykrywalny: nieaktualne odczyty w 1/3 prób (przed poprawką 3/3) |

**Porównanie `bridge-churn` przed i po** (ta sama konfiguracja, po 20 prób, `results/lost-write-2026-10-10/{before,after}-fix/bridge-churn-20`):

| Build | Ok | Unknown | Próby z utraconym zapisem (na pewno nieaktualne odczyty) |
|---|---|---|---|
| Przed poprawką | 12 | 8 | 1 (próba 7: ten sam wzorzec, restartowany most n4 z krótszym logiem wygrał elekcję) |
| Po poprawce | 17 | 3 | 0 |

Razem z pierwotnym przebiegiem: przed poprawką utrata w 2 z 23 prób, po poprawce w 0 z 20.

## 7. Ograniczenia i niepewności

- **Różnica 2/23 wobec 0/20 nie jest statystycznie rozstrzygająca.** Dowodem poprawki są testy deterministyczne i instrumentacja, nie te przebiegi.
- **Wyniki `Unknown` są nierozstrzygnięte:** 3 po poprawce, 8 przed. Nie liczę ich jako sukcesów. Skan nieaktualnych odczytów jest zachowawczy i nie zastępuje checkera.
- **Wykrywalność ackbeforefsync spadła z 3/3 do 1/3.** Nie zbadałem, czy to przypadek przy 3 próbach, czy skutek zmiany w czasie odpowiedzi.
- **Wpływ na wydajność nie został zmierzony.** Heartbeat follower'a czeka teraz na trwający zapis grupowy. E3/E7 trzeba powtórzyć na nowym kodzie.
- **Pozostałe punkty z deep dive nie są poprawione:**
  - 16.2: zapis kadencji w `becomeFollower` nie jest oczekiwany przed odpowiedzią.
  - Follower stosuje zatwierdzone wpisy, zanim je u siebie zapisze.
  - Przy poprawnym liczeniu większości nie łamie to bezpieczeństwa [wniosek], ale nie ma na to testu.
- **Rozdartego końca WAL-a nie da się wykluczyć z logów, bo jest obcinany po cichu.** Nie jest jednak potrzebny do wyjaśnienia utraty.
