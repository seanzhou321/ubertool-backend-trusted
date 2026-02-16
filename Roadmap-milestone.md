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

1. (done) email notifications
2. (done) Build a cron job engine for below tasks

   * mark the overdue rentals OVERDUE (nightly)
   * send overdue reminders to renter (nightly)
   * Apply platform enforced judgements against unresolved disputes at the end of the month before balance snapshots (monthly)
   * Take balance snapshot at the end of the week before bill splitting operation (monthly)
   * perform monthly bill splitting operation (monthly)
   * send bill splitting notice reminders to both creditors and debtors regarding to the unresolved bills (nightly)

3. (done) Build UI screen and api to handle bill payment acknowledgements and ledger transactions
4. Add legal statements:

   * Privacy policy
   * Terms of service
   * rental pricing and cost agreement confirmation
   * community bylaws
   * Dispute resolution policy

5. enable refresh token and comprehensive manual testing
6. enable 2fa

## Milestone 3: Production - trial groups

1. https certification
2. Form a company
3. deploy backend to cloud and frontend to Google Play Store
4. release to selected communities
5. Port frontend code into iOS app and release it to Apple App Store

## Milestone 4: Enhancements

1. image capture and display
2. passkey
3. push notification through FCM (Firebase Cloud Messaging, a free messaging service from Google, delivers to both Android and iOS devices)
4. Set up Patreon/support us contribution process.
5. Setup feedback collection links.

## Milestone 5: General production release

1. Self registering a community/org endorsed by an existing SUPER_ADMIN or by the app platform. 
2. Production release to general public
3. Apply small business innovation grants from Amazon, Google, Microsoft, Meta, Salesforce, etc. 

## Milestone 6: Ad Service integration

1. recommendation and personalization engine for advertisement

## Milestone 7: Vicinity Based Tool Search for Prime Accounts

1. Prime Member subscription. Prime Member can show their tools and services to renters search list based on the vicinity rules.  



