import $api from "common/api";
import { $gettext } from "common/gettext";
import Rest from "model/rest";
import Photo from "model/photo";
import { Thumb } from "model/thumb";

// Cull models a near-duplicate / burst cull group over multiple photos.
export class Cull extends Rest {
  getDefaults() {
    return {
      ID: 0,
      UID: "",
      KeeperUID: "",
      MemberCount: 0,
      CullScore: 0,
      CullSrc: "",
      ReviewedAt: null,
      CreatedAt: "",
      UpdatedAt: "",
      Thumb: "",
      Members: [],
    };
  }

  getEntityName() {
    return this.UID;
  }

  getTitle() {
    return $gettext("Near Duplicates");
  }

  classes() {
    return ["is-cull", "uid-" + this.UID];
  }

  // thumbnailUrl returns a preview URL for the keeper when Hash is known.
  thumbnailUrl(size) {
    if (this.Thumb) {
      return new Photo({ Hash: this.Thumb }).thumbnailUrl(size || "tile_224");
    }
    const keeper = (this.Members || []).find((m) => m.PhotoUID === this.KeeperUID || m.MemberRole === "keeper");
    if (keeper?.Photo) {
      return new Photo(keeper.Photo).thumbnailUrl(size || "tile_224");
    }
    return `${window.__CONFIG__?.contentUri || ""}/svg/photo`;
  }

  // openReview loads members and opens them in the lightbox.
  openReview(lightbox) {
    return this.find(this.UID).then((cull) => {
      const photos = [];
      for (const m of cull.Members || []) {
        if (m.Photo) {
          photos.push(new Photo(m.Photo));
        }
      }

      const thumbs = Thumb.fromPhotos(photos);
      if (thumbs.length && lightbox) {
        lightbox.openModels(thumbs, 0);
      }

      return cull;
    });
  }

  setKeeper(photoUID) {
    return $api.post(this.getEntityResource() + "/keeper", { PhotoUID: photoUID }).then((r) => this.setValues(r.data));
  }

  protect(photoUID) {
    return $api.post(this.getEntityResource() + "/keep", { PhotoUID: photoUID }).then((r) => this.setValues(r.data));
  }

  restoreGroup() {
    return $api.post(this.getEntityResource() + "/restore");
  }

  dissolve() {
    return $api.delete(this.getEntityResource());
  }

  static getCollectionResource() {
    return "culls";
  }

  static getModelName() {
    return $gettext("Cull");
  }
}

export default Cull;
