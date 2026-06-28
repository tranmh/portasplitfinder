// Allow importing image assets (used for the Leaflet marker-icon fix).
declare module "*.png" {
  const content: import("next/image").StaticImageData;
  export default content;
}
