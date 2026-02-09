# Bill Splitting Process and Dispute Resolution

## Bill Splitting Process

At the conclusion of each rental contract, the renter will be debited and the tool owner will be credited with the agreed-upon price. At the end of each month, the app will calculate the simplest payment arrangement for members within an organization to settle their accounts. As a result of this calculation, net debtors will pay net creditors.

Once payment amounts for each debtor are determined, the app will send notices to all debtors. After debtors pay creditors using their mutually agreed-upon payment method, debtors must acknowledge their payments in the app. A notice will then be sent to the creditors to confirm receipt of payment. Once creditors acknowledge receipt, both debtors' and creditors' accounts will be updated to reflect the payment amounts.

## Dispute Scenarios

This process works smoothly when every debtor pays and every creditor acknowledges receipt. However, if payments are not resolved within 10 days after the initial notice is sent, the app will classify the payment as in-dispute.

**Two scenarios trigger a payment dispute:**

1. The debtor does not acknowledge submission of the payment.
2. The debtor acknowledges submission of the payment; however, the creditor does not acknowledge receipt.

## End States of Dispute Resolution

A dispute can be resolved in three ways:

1. Both the debtor and creditor acknowledge submission and receipt of payment. This represents a graceful resolution of the dispute.
2. The debtor acknowledges submission of payment; however, the creditor fails to acknowledge receipt.
3. The debtor does not acknowledge submission of payment.

## Admin and Super Admin Mediation

When a payment dispute cannot be resolved between the parties involved, admin and/or super admin intervention is required. If mediation cannot achieve a graceful resolution, a forced resolution will be imposed by the app at the end of the month.

**Three possible outcomes of forced resolutions:**

1. **Debtor is at fault.** The debtor will be blocked from submitting rental requests until all debts are cleared. The bill splitting algorithm only considers bills among active members of the community. Therefore, the creditor's credit will be paid by other active members in the community.

2. **Creditor is at fault.** The admin/super admin should mark the payment as valid. The admin/super admin may also determine the creditor's continued status within the community.

3. **Both parties are at fault.** The debtor will be blocked from renting and the creditor will be blocked from lending tools within the community.

The admin/super admin must determine which outcome applies when a resolution cannot be reached. If no admin/super admin action is taken on an unresolved disputed payment, the app will automatically block both the creditor and debtor from lending and renting within the organization. This default decision will be triggered at the end of the month and can be reversed by admin/super admin action.
