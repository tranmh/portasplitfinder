import type { Messages } from "./de";

export const en: Messages = {
  appTitle: "PortaSplitFinder",
  appSubtitle: "Availability of Portasplit air conditioners near you",

  filter: {
    heading: "Location & radius",
    plzLabel: "Postal code (PLZ)",
    plzPlaceholder: "e.g. 10115",
    plzInvalid: "Please enter a valid 5-digit postal code.",
    radiusLabel: "Radius",
    radiusUnit: "km",
    reset: "Reset",
    inStockOnly: "Show only stores with availability",
  },

  map: {
    inStock: "In stock",
    outOfStock: "Unavailable",
    yourLocation: "Your location",
    loading: "Loading map …",
  },

  stores: {
    heading: "Nearby stores",
    none: "No stores found within the selected radius.",
    noCenter: "Enter a postal code to sort stores by distance.",
    distance: "Distance",
    inStock: "In stock",
    outOfStock: "Unavailable",
    viewListing: "View listing",
  },

  storeSearch: {
    heading: "Find a store",
    placeholder: "Store name or city",
    noResults: "No matching stores found.",
    results: "results",
    loadMore: "Load more stores",
    error: "Search failed. Please try again.",
  },

  online: {
    heading: "Online retailers",
    nationwide: "available nationwide",
    none: "No online offers at the moment.",
    priceUnknown: "Price unknown",
    inStock: "In stock",
    outOfStock: "Unavailable",
    lastChecked: "Last checked",
    never: "never",
    viewListing: "Go to shop",
  },

  subscribe: {
    heading: "Get notified when available",
    description:
      "Receive an email as soon as a device becomes available in your area.",
    emailLabel: "Email address",
    emailPlaceholder: "you@email.com",
    plzLabel: "PLZ",
    plzPlaceholder: "10115",
    radiusLabel: "Radius (km)",
    submit: "Notify me",
    submitting: "Sending …",
    success: "Confirmation email sent",
    successDetail: "Please confirm your subscription via the link in the email.",
    emailInvalid: "Please enter a valid email address.",
    plzInvalid: "Please enter a valid 5-digit postal code.",
    radiusInvalid: "Radius must be between 1 and 500 km.",
    genericError: "Something went wrong. Please try again.",
  },

  state: {
    loading: "Loading data …",
    error: "Could not load data.",
    retry: "Retry",
    empty: "No availability found.",
    generatedAt: "As of",
  },
};
