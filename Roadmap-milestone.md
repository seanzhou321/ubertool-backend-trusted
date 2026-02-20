# Ubertool Milestone and Roadmap

## Milestone 1: Demo Release

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

   * Privacy policy - based on Facebook
   * Terms of service - based on Facebook
   * rental pricing and cost agreement confirmation
   * community bylaws
   * Dispute resolution policy

5. enable refresh token and comprehensive manual testing
6. enable 2fa

## Milestone 3: MVP Production Release - Limited to Trial Groups

1. https certification
2. Form a company
3. deploy backend to cloud and frontend to Google Play Store
4. release to selected communities
5. Port frontend code into the iOS app and release it to Apple App Store

## Milestone 4: Enhancements

1. image capture and display
2. passkey
3. push notification through FCM (Firebase Cloud Messaging, a free messaging service from Google, delivers to both Android and iOS devices)
4. Set up Patreon/GoFundMe/support us contribution process.
5. Setup feedback collection links.
6. Impose $300 ceiling rule. 
7. Enforce the data deletion 30 days after user unsubscribe without balance and pending disputes. 
8. Copying the rental prices to rental request at the creation. Use the pricing in the rental request to derive final credit. show the rental request price throughout the rental state transitions. 
9. Implement user management rules: super_admin creates/promotes admin and member. admin creates members. 

## Milestone 5: General Production Release

1. Self registering a community/org endorsed by an existing SUPER_ADMIN, a community leader, or by the app platform. 
2. Implement legally required data retention processes.
3. Implement support of legal updates.
4. Production release to general public
5. Apply small business innovation grants from Amazon, Google, Microsoft, Meta, Salesforce, etc. 

## Milestone 6: Vicinity Based Tool Search for Prime Accounts

1. Prime Member subscription. Prime Member can show their tools and services to renters search list based on the vicinity rules.  
2. Prime Members are allowed to rent items without $300 limit.

## Milestone 7: Ad Service integration

1. Recommendation and personalization engine for advertisement

## Milestone 8: Feature upgrades

1. Create adult supervised minor accounts for toy lending
2. Create adult supervised junior accounts for general item lending.


