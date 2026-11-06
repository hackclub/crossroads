// The one long train on Home, Shop and Boxes (see RailTrain.svelte).
//
// Home and Shop hand their live car content to the train with <Station>;
// while a page isn't open, its car shows a small preview instead. The stamp
// car loads its own stamps; bump stampsVersion to make it reload them.
export const rail = $state({
	home: null, // snippet from the Home page, while it's open
	shop: null, // snippet from the Shop page, while it's open
	stampsVersion: 0,
	// Sequencing the train with the Repair Bridge above it (CommunityBridge),
	// as performance.now() times: when the train comes to rest after rolling
	// in or moving, and when the bridge has finished folding away. The bridge
	// opens only once the train has stopped; the train leaves Home only once
	// the bridge has folded.
	settle: 0,
	foldUntil: 0
});

// How long the bridge takes to fold away (its closing transitions in
// CommunityBridge.svelte end at 0.8s).
export const BRIDGE_FOLD_MS = 800;

// Front of the train on the right; it moves right to reach cars further back.
export const STATIONS = ['/boxes', '/shop', '/home'];
