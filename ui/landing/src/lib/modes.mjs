// The one set of approval-mode names a person sees.
// Publishers, setup, and /you/ use these words. The stored ids stay here.

export const approvalModes = [
  { id: 'manual', name: 'Ask me every time', note: 'Every placement waits for your yes.' },
  { id: 'rules', name: 'Approve within my rules', note: 'Allowed categories, minimum relevance, positions, a 30-day cap.' },
  { id: 'checks', name: 'Approve anything that passes the checks', note: 'Anything that passes the checks can go live. Undo is one tap.' },
]

export const personApprovesLine = `With ${approvalModes[0].name}, a person approves each placement, and with ${approvalModes[1].name} and ${approvalModes[2].name}, the agent does.`

export const signInStory = 'Agents join with no sign-in. People sign in with Google for the dashboard.'

export const personSignsInBecause = 'A person signs in with Google so the dashboard can save the site and the approval mode. Agents join with no sign-in.'
