package llm

// historicalBBSSubjectCalibration is prompt guidance derived from preserved
// Japanese PC-communication subject-line corpora. It intentionally describes
// the breadth of observed subject-field usage without turning that evidence into
// a template bank, style enum, or target percentage distribution.
const historicalBBSSubjectCalibration = `Preserved Japanese PC-communication subject-line corpora show a much wider range than modern Q&A-style headlines. Use that evidence as calibration, not as a template bank or target distribution.

- For a root post, generate the exact string this actor would type into the BBS subject field at this moment, at most 36 characters.
- The subject does NOT need to summarize the body, explain the topic to strangers, be a complete sentence, or be useful as a search result.
- Depending on the actor and context, it may naturally be a very short interjection or fragment, a noun/topic label, direct address, continuation shorthand, personal update, announcement, reaction, joke, or question.
- Do not default to polite survey/request forms such as "...いますか", "...どうですか", or "...しませんか". Use a question only when asking is actually the actor's goal.
- Preserve individual voice using the persistent persona and prior visible behavior. Do not invent a fixed subject-style parameter just to manufacture variety.
- Compare the root subjects in this batch with recent supplied subjects. If several have accidentally converged on the same rhetorical construction or ending, reconsider only those for which another equally natural wording follows from the actor and event. Do not force every subject to differ.
- Do NOT rotate through categories, enforce quotas, or copy a stored historical subject. Historical corpus evidence is calibration for breadth, not a phrase bank.`
