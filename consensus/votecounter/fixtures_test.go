package votecounter

var simpleVotingPower = buildVotingPower(10, 6, 4, 4, 2, 5)

var simpleMockValidator = newMockValidator(2, simpleVotingPower)

var simpleTestScenarios = []testCase{
	// Voter 0 is honest, send each vote for ID 0 twice
	voter(0).prevote(0),
	voter(0).prevote(0).expectSkip(),
	voter(0).precommit(0),
	voter(0).precommit(0).expectSkip(),

	// Voter 1 is also honest, send each vote for ID 0 once
	voter(1).precommit(0),
	voter(1).prevote(0),

	// Receive equivocated proposal 0 twice from voter 2
	voter(2).propose(0),
	voter(2).propose(0).expectSkip(),
	voter(2).propose(1).expectSkip(),
	voter(2).propose(1).expectSkip(),

	// Voter 2 is dishonest, prevote ID 0 and ID 1, precommit ID 1 and nil
	voter(2).prevote(0),
	voter(2).prevote(1),
	voter(2).precommit(1),
	voter(2).precommitNil(),

	// Receive equivocated proposal 1 again from voter 2
	voter(2).propose(1).expectSkip(),

	// Voter 3 is dishonest, prevote ID 1 and nil, precommit ID 0 and 1
	voter(3).prevote(1),
	voter(3).prevoteNil(),
	voter(3).precommit(0),
	voter(3).precommit(1),

	// Voter 4 is honest, prevote ID 0 and precommit nil
	voter(4).prevote(0),
	voter(4).precommitNil(),

	// Voter 5 is honest, prevote nil and precommit ID 0
	voter(5).prevoteNil(),
	voter(5).precommit(0),
}
