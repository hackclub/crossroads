// The one long train on Home, Shop and Boxes (see RailTrain.svelte).
//
// Home and Shop hand their live car content to the train with <Station>;
// while a page isn't open, its car shows the same car drawn from what was
// last loaded (cars/HomePreview, cars/ShopPreview), so a car looks the same
// before, during and after the train moves. The stamp car loads its own
// stamps; bump stampsVersion to make it reload them.
export const rail = $state({
	home: null, // snippet from the Home page, while it's open
	shop: null, // snippet from the Shop page, while it's open
	stampsVersion: 0,
	// Whether they've moved between pages since the site loaded (the layout
	// sets it). The train only rolls in when it arrives that way.
	traveled: false
});

// The cars in order, left to right: the same order as Home, Shop and Boxes in
// the top bar, so the train moves the way the nav reads. Home's car is the
// front of the train, on the left.
export const STATIONS = ['/home', '/shop', '/boxes'];
