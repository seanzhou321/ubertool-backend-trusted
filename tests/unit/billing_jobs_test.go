package unit

import (
	"testing"

	"ubertool-backend-trusted/internal/jobs"
)

func TestCalculateTransactions(t *testing.T) {
	// Example: Small church community tool sharing
	// Scenarios from reference implementation
	accounts := []jobs.Account{
		{UserID: 1, Balance: 4550},   // John
		{UserID: 2, Balance: -3820},  // Mary
		{UserID: 3, Balance: 1275},   // Peter
		{UserID: 4, Balance: -1560},  // Sarah
		{UserID: 5, Balance: 320},    // David (Below threshold 500)
		{UserID: 6, Balance: -280},   // Emma  (Below threshold 500)
		{UserID: 7, Balance: 2500},   // Luke
		{UserID: 8, Balance: -3015},  // Anna
		{UserID: 9, Balance: 450},    // Mark  (Below threshold 500)
		{UserID: 10, Balance: -420},  // Ruth  (Below threshold 500)
	}

	threshold := 500 // $5.00

	var creditors, debtors []jobs.Account
	for _, acc := range accounts {
		if acc.Balance > 0 {
			creditors = append(creditors, acc)
		} else if acc.Balance < 0 {
			debtors = append(debtors, acc)
		}
	}

	transactions := jobs.CalculateTransactions(creditors, debtors, threshold)

	// Verify results
	// 1. Calculate total settled amount
	totalSettled := 0
	for _, txn := range transactions {
		totalSettled += txn.Amount
	}

	// 2. Identify remaining balances
	// Reconstruct state after transactions
	finalBalances := make(map[int]int)
	for _, acc := range accounts {
		finalBalances[acc.UserID] = acc.Balance
	}
	for _, txn := range transactions {
		finalBalances[txn.FromUserID] += txn.Amount // Debtor pays, so balance increases (less negative)
		finalBalances[txn.ToUserID] -= txn.Amount   // Creditor receives, so balance decreases (less positive)
	}

	// Check if small accounts are untouched (or settled if matched with large ones?)
	// The algorithm matches largest with largest.
	// Let's trace expected behavior for small accounts:
	// David (320), Emma (-280), Mark (450), Ruth (-420). All < 500.
	// Since there are larger accounts, the loop continues.
	// Eventually, large accounts might be reduced to small amounts?
	// OR, small accounts might be matched if they are the largest REMAINING.
	// BUT, if both top creditor and top debtor are < 500, we stop.
	
	// Let's see what happens.
	// Sort by abs:
	// Creditors: John(4550), Luke(2500), Peter(1275), Mark(450), David(320)
	// Debtors: Mary(-3820), Anna(-3015), Sarah(-1560), Ruth(-420), Emma(-280)

	// 1. John(4550) vs Mary(3820). Both > 500. Match 3820. John rem=730. Mary=0.
	// 2. Anna(3015) vs Luke(2500). Both > 500. Match 2500. Anna rem=515. Luke=0.
	// 3. Sarah(1560) vs Peter(1275). Both > 500. Match 1275. Sarah rem=285. Peter=0.
	// 4. John(730) vs Anna(515). Both > 500. Match 515. John rem=215. Anna=0.
	
	// Remaining Heaps:
	// Creditors: Mark(450), David(320), John(215)  <-- All < 500
	// Debtors: Ruth(420), Sarah(285), Emma(280)    <-- All < 500
	
	// Top Creditor: Mark(450) < 500
	// Top Debtor: Ruth(420) < 500
	// Loop terminates.
	
	// So, Mark, David, John(remainder), Ruth, Sarah(remainder), Emma should remain unsettled.
	
	// Verify total transactions amount
	// 3820 + 2500 + 1275 + 515 = 8110
	if totalSettled != 8110 {
		t.Errorf("Expected total settled 8110, got %d", totalSettled)
	}
	
	// Verify remaining balances
	remainingCreditorsTotal := 0
	remainingDebtorsTotal := 0
	
	for _, bal := range finalBalances {
		if bal > 0 {
			remainingCreditorsTotal += bal
		} else {
			remainingDebtorsTotal += bal
		}
	}
	
	// Expected remaining creditors: 450(Mark) + 320(David) + 215(John) = 985
	if remainingCreditorsTotal != 985 {
		t.Errorf("Expected remaining creditors total 985, got %d", remainingCreditorsTotal)
	}
	
	// Expected remaining debtors: -420(Ruth) - 285(Sarah) - 280(Emma) = -985
	if remainingDebtorsTotal != -985 {
		t.Errorf("Expected remaining debtors total -985, got %d", remainingDebtorsTotal)
	}
}

func TestCalculateTransactions_EdgeCase(t *testing.T) {
	// Alice(5000) vs 10 borrowers of 300 each. Threshold 500.
	// Creditor: Alice(5000)
	// Debtors: 10x Borrower(-300)
	
	// Loop 1: Alice(5000) vs B1(300). Alice > 500? Yes. B1 < 500.
	// Condition "Both < Threshold" is FALSE.
	// So we continue!
	// Match min(5000, 300) = 300.
	// Alice becomes 4700. B1 becomes 0.
	
	// Loop 2...10: Alice consumes all small debtors.
	
	var creditors, debtors []jobs.Account
	creditors = append(creditors, jobs.Account{UserID: 1, Balance: 5000})
	
	for i := 0; i < 10; i++ {
		debtors = append(debtors, jobs.Account{UserID: 10 + i, Balance: -300})
	}
	
	transactions := jobs.CalculateTransactions(creditors, debtors, 500)
	
	totalSettled := 0
	for _, txn := range transactions {
		totalSettled += txn.Amount
	}
	
	if totalSettled != 3000 {
		t.Errorf("Expected total settled 3000, got %d", totalSettled)
	}
}
