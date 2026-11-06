// The one long train on Home, Shop and Boxes (see RailTrain.svelte).
//
// Home and Shop hand their live car content to the train with <Station>;
// while a page isn't open, its car shows a small preview instead. The stamp
// car loads its own stamps; bump stampsVersion to make it reload them.
export const rail = $state({
	home: null, // snippet from the Home page, while it's open
	shop: null, // snippet from the Shop page, while it's open
	stampsVersion: 0
});

// Front of the train on the right; it moves right to reach cars further back.
export const STATIONS = ['/boxes', '/shop', '/home'];
