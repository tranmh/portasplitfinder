export const de = {
  appTitle: "PortaSplitFinder",
  appSubtitle: "Verfügbarkeit von Portasplit-Klimageräten in Ihrer Nähe",

  filter: {
    heading: "Standort & Umkreis",
    plzLabel: "Postleitzahl (PLZ)",
    plzPlaceholder: "z. B. 10115",
    plzInvalid: "Bitte eine gültige 5-stellige PLZ eingeben.",
    radiusLabel: "Umkreis",
    radiusUnit: "km",
    reset: "Zurücksetzen",
    inStockOnly: "Nur Märkte mit Verfügbarkeit anzeigen",
  },

  map: {
    inStock: "Auf Lager",
    outOfStock: "Nicht verfügbar",
    yourLocation: "Ihr Standort",
    loading: "Karte wird geladen …",
  },

  stores: {
    heading: "Märkte in Ihrer Nähe",
    none: "Keine Märkte im gewählten Umkreis gefunden.",
    noCenter: "Geben Sie eine PLZ ein, um Märkte nach Entfernung zu sortieren.",
    distance: "Entfernung",
    inStock: "Auf Lager",
    outOfStock: "Nicht verfügbar",
    viewListing: "Zum Angebot",
  },

  storeSearch: {
    heading: "Markt suchen",
    placeholder: "Marktname oder Stadt",
    noResults: "Keine passenden Märkte gefunden.",
    results: "Treffer",
    loadMore: "Weitere Märkte laden",
    error: "Suche fehlgeschlagen. Bitte erneut versuchen.",
  },

  online: {
    heading: "Online-Händler",
    nationwide: "deutschlandweit verfügbar",
    none: "Derzeit keine Online-Angebote.",
    priceUnknown: "Preis unbekannt",
    inStock: "Auf Lager",
    outOfStock: "Nicht verfügbar",
    lastChecked: "Zuletzt geprüft",
    never: "noch nie",
    viewListing: "Zum Shop",
  },

  subscribe: {
    heading: "Benachrichtigung bei Verfügbarkeit",
    description:
      "Erhalten Sie eine E-Mail, sobald ein Gerät in Ihrem Umkreis verfügbar ist.",
    emailLabel: "E-Mail-Adresse",
    emailPlaceholder: "ihre@email.de",
    plzLabel: "PLZ",
    plzPlaceholder: "10115",
    radiusLabel: "Umkreis (km)",
    submit: "Benachrichtigen lassen",
    submitting: "Wird gesendet …",
    success: "Bestätigungs-E-Mail gesendet",
    successDetail:
      "Bitte bestätigen Sie Ihre Anmeldung über den Link in der E-Mail.",
    emailInvalid: "Bitte eine gültige E-Mail-Adresse eingeben.",
    plzInvalid: "Bitte eine gültige 5-stellige PLZ eingeben.",
    radiusInvalid: "Umkreis muss zwischen 1 und 500 km liegen.",
    genericError: "Etwas ist schiefgelaufen. Bitte versuchen Sie es erneut.",
  },

  state: {
    loading: "Daten werden geladen …",
    error: "Daten konnten nicht geladen werden.",
    retry: "Erneut versuchen",
    empty: "Keine Verfügbarkeit gefunden.",
    generatedAt: "Stand",
  },
};

// Recursively widen literal string values so other locales can supply their
// own copy while keeping the same shape.
type DeepStringShape<T> = {
  [K in keyof T]: T[K] extends string ? string : DeepStringShape<T[K]>;
};

export type Messages = DeepStringShape<typeof de>;
