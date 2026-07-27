<template>
  <div ref="page" tabindex="-1" class="p-page p-page-culls not-selectable">
    <v-toolbar flat :density="$vuetify.display.smAndDown ? 'compact' : 'default'" color="secondary" class="page-toolbar">
      <v-toolbar-title>
        {{ $gettext(`Near Duplicates`) }}
      </v-toolbar-title>
      <v-spacer></v-spacer>
      <v-btn icon variant="text" :title="$gettext('Refresh')" class="action-reload" @click="refresh()">
        <v-icon>mdi-refresh</v-icon>
      </v-btn>
    </v-toolbar>

    <div v-if="loading" class="p-page__loading">
      <p-loading></p-loading>
    </div>
    <div v-else class="p-page__content pa-3">
      <div v-if="results.length === 0">
        <v-alert color="surface-variant" icon="mdi-checkbox-multiple-outline" class="no-results" variant="outlined">
          <div class="font-weight-bold">
            {{ $gettext(`No near-duplicate groups found`) }}
          </div>
          <div class="mt-2">
            {{ $gettext(`Run photoprism cull run after indexing, or wait for the scheduled cull worker.`) }}
          </div>
        </v-alert>
      </div>
      <div v-else class="v-row search-results cards-view">
        <div v-for="(cull, index) in results" :key="cull.UID" class="v-col-6 v-col-sm-4 v-col-md-3 v-col-xl-2">
          <div :data-uid="cull.UID" class="result not-selectable" :class="cull.classes()" @click="openCull(index)">
            <div
              class="preview"
              :style="previewStyle(cull)"
              :title="$gettext('Review Near Duplicates')"
            >
              <div class="preview__overlay"></div>
              <div class="preview-details">
                <div class="info-text">{{ cull.MemberCount }}</div>
              </div>
            </div>
            <div class="meta">
              <div class="meta-title">
                {{ cull.KeeperUID }}
              </div>
              <div class="meta-subtitle">
                <span v-if="cull.ReviewedAt">{{ $gettext(`Reviewed`) }}</span>
                <span v-else>{{ $gettext(`Unreviewed`) }}</span>
                · {{ cull.CullSrc }}
              </div>
            </div>
            <div class="d-flex justify-space-between mt-1">
              <v-btn size="small" variant="text" class="action-restore" @click.stop="restoreCull(cull)">
                {{ $gettext(`Restore`) }}
              </v-btn>
              <v-btn size="small" variant="text" color="error" class="action-dissolve" @click.stop="dissolveCull(cull)">
                {{ $gettext(`Dissolve`) }}
              </v-btn>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Cull from "model/cull";
import { $gettext } from "common/gettext";

export default {
  name: "PPageCulls",
  data() {
    return {
      loading: true,
      results: [],
    };
  },
  mounted() {
    this.refresh();
  },
  methods: {
    refresh() {
      this.loading = true;
      Cull.search({ count: 100, offset: 0 })
        .then((response) => {
          this.results = (response.models || []).map((m) => (m instanceof Cull ? m : new Cull(m)));
        })
        .finally(() => {
          this.loading = false;
        });
    },
    previewStyle(cull) {
      // Keeper hash is only present after GetCull; list uses SVG placeholder.
      return { backgroundImage: `url(${cull.thumbnailUrl("tile_224")})` };
    },
    openCull(index) {
      const cull = this.results[index];
      if (!cull) {
        return;
      }
      cull.openReview(this.$lightbox).then((loaded) => {
        this.results.splice(index, 1, loaded);
      });
    },
    restoreCull(cull) {
      cull.restoreGroup().then(() => {
        this.$notify.success($gettext("Restored"));
        this.refresh();
      });
    },
    dissolveCull(cull) {
      cull.dissolve().then(() => {
        this.$notify.success($gettext("Dissolved"));
        this.refresh();
      });
    },
  },
};
</script>
