# Ubertool Milestone and Roadmap

## Milestone 1: Demo
1. (done) Dashboard balance in $
2. (done) Add overall balance across all orgs.
3. (done) Redo pricing and rental cost instructions in Add New Tool screen. 
4. (done) Add rental cost agreement in the renter confirmation screen.
5. (done) Add pickup button and move the rental request from SCHEDULED to ACTIVE state.
6. (done) Add change/extend rental period capability
7. (done) Add rental history to the tool detail screen. 
8. (done) Add return condition logging and repair/replacement charge screen at the rental completion stage.
9. (done) Add admin name/email to the signon Request screen/api

## Milestone 2: Functional Feature Complete
1. email notifications
2. Build a cron job engine for below tasks
- mark the overdue rentals OVERDUE (nightly)
- send overdue reminders to renter (nightly)
- Apply platform enforced judgements against unresolved disputes at the end of the month before balance snapshots (monthly)
- Take balance snapshot at the end of the week before bill splitting operation (monthly)
- perform monthly bill splitting operation (monthly)
- send bill splitting notice reminders to both creditors and debtors regarding to the unresolved bills (nightly)
3. Build UI screen and api to handle bill payment acknowledgements and ledger transactions
4. Add legal statements: community bylaws and rental pricing and cost agreement confirmation
5. enable refresh token
6. enable 2fa

## Milestone 3: Production - trial groups
1. https certification
2. deploy to cloud
3. release to selected communities

## Milestone 4: Enhancements
1. iOS app and release
2. Form a company
3. image capture and display
4. passkey
5. push notification through FCM

## Milestone 5: General production release
1. Self registrating a community/org endorsing a SUPER_ADMIN
2. Production release to general public

## Milestone 6: Ad Service integration
1. add recomendations by advertisement
