import torch
import torch.nn as nn
from torchvision.models import efficientnet_b0, efficientnet_b3

from .constants import LEAD_NAMES, LEAD_TO_IDX, LAYOUT_FAMILY_TO_IDX

RHYTHM_SUPER_CLASSES = ["SINUS", "ATRIAL", "VENTRICULAR", "PACE", "OTHER"]
ATRIAL_AUX_CLASSES = ["AFIB", "AFLT", "SVTAC"]

class ECGV9ContextResidual(nn.Module):
    def __init__(self, n_rhythm, n_binary, n_layout, d_model=192, dropout=0.25):
        super().__init__()

        self.backbone = efficientnet_b3(weights=None)
        self.full_dim = self.backbone.classifier[-1].in_features
        self.backbone.classifier = nn.Identity()

        self.lead_backbone = efficientnet_b0(weights=None)
        self.lead_dim = self.lead_backbone.classifier[-1].in_features
        self.lead_backbone.classifier = nn.Identity()

        self.full_proj = nn.Sequential(
            nn.Linear(self.full_dim, d_model),
            nn.LayerNorm(d_model),
            nn.Dropout(dropout)
        )

        self.lead_proj = nn.Sequential(
            nn.Linear(self.lead_dim, d_model),
            nn.LayerNorm(d_model),
            nn.Dropout(dropout)
        )

        self.lead_id_embed = nn.Embedding(len(LEAD_NAMES), d_model)
        self.layout_embed = nn.Embedding(len(LAYOUT_FAMILY_TO_IDX), d_model)

        self.box_mlp = nn.Sequential(
            nn.Linear(4, d_model),
            nn.GELU(),
            nn.Linear(d_model, d_model)
        )

        self.real_mask_mlp = nn.Sequential(
            nn.Linear(1, d_model),
            nn.GELU(),
            nn.Linear(d_model, d_model)
        )

        self.strip_flag_mlp = nn.Sequential(
            nn.Linear(1, 16),
            nn.GELU(),
            nn.Linear(16, 16)
        )

        self.cls_token = nn.Parameter(torch.zeros(1, 1, d_model))

        encoder_layer = nn.TransformerEncoderLayer(
            d_model=d_model,
            nhead=6,
            dim_feedforward=d_model * 4,
            dropout=dropout,
            activation="gelu",
            batch_first=True,
            norm_first=True
        )
        self.transformer = nn.TransformerEncoder(encoder_layer, num_layers=2)
        self.post_norm = nn.LayerNorm(d_model)

        rhythm_fusion_dim = d_model * 6 + 16
        atrial_fusion_dim = d_model * 4 + 16
        pace_fusion_dim = d_model * 3 + 16
        binary_fusion_dim = d_model * 3
        super_fusion_dim = d_model * 3
        layout_fusion_dim = d_model * 2

        self.rhythm_head = nn.Sequential(
            nn.Linear(rhythm_fusion_dim, 512),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(512, n_rhythm)
        )

        self.rhythm_super_head = nn.Sequential(
            nn.Linear(super_fusion_dim, 256),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(256, len(RHYTHM_SUPER_CLASSES))
        )

        self.atrial_aux_head = nn.Sequential(
            nn.Linear(atrial_fusion_dim, 256),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(256, len(ATRIAL_AUX_CLASSES))
        )

        self.pace_aux_head = nn.Sequential(
            nn.Linear(pace_fusion_dim, 128),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(128, 1)
        )

        self.binary_head = nn.Sequential(
            nn.Linear(binary_fusion_dim, 512),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(512, n_binary)
        )

        self.layout_head = nn.Sequential(
            nn.Linear(layout_fusion_dim, 256),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(256, n_layout)
        )

        self.tile_backbone = efficientnet_b0(weights=None)
        tile_dim = self.tile_backbone.classifier[-1].in_features
        self.tile_backbone.classifier = nn.Identity()

        self.tile_proj = nn.Sequential(
            nn.Linear(tile_dim, d_model),
            nn.LayerNorm(d_model),
            nn.GELU(),
            nn.Dropout(dropout)
        )

        self.tile_attn = nn.Sequential(
            nn.Linear(d_model, d_model // 2),
            nn.Tanh(),
            nn.Linear(d_model // 2, 1)
        )

        self.context_rhythm_residual = nn.Sequential(
            nn.Linear(d_model * 2, 256),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(256, n_rhythm)
        )

        self.context_gate = nn.Parameter(torch.tensor(0.0))
        self._init_buffers()

    def _init_buffers(self):
        self.register_buffer("idx_strip", torch.tensor([LEAD_TO_IDX["RHYTHM_STRIP"]], dtype=torch.long))
        self.register_buffer("idx_ii", torch.tensor([LEAD_TO_IDX["II"]], dtype=torch.long))
        self.register_buffer("idx_v1", torch.tensor([LEAD_TO_IDX["V1"]], dtype=torch.long))
        self.register_buffer("idx_inferior", torch.tensor([
            LEAD_TO_IDX["II"], LEAD_TO_IDX["III"], LEAD_TO_IDX["aVF"]
        ], dtype=torch.long))

    def pool_idx(self, x, idx):
        return x.index_select(1, idx).mean(dim=1)

    def forward(self, x_full, x_tiles, x_leads, lead_boxes, lead_id_idx, lead_real_mask, layout_idx, has_rhythm_strip):
        B, T, C, Ht, Wt = x_tiles.shape
        _, N, _, Hl, Wl = x_leads.shape

        full_feats = self.backbone(x_full)
        page_tok = self.full_proj(full_feats)

        x_flat = x_leads.view(B * N, 3, Hl, Wl)
        lead_feats = self.lead_backbone(x_flat)
        lead_feats = self.lead_proj(lead_feats).view(B, N, -1)

        lead_id_emb = self.lead_id_embed(lead_id_idx)
        layout_emb = self.layout_embed(layout_idx)
        layout_emb_exp = layout_emb.unsqueeze(1).expand(B, N, -1)
        box_emb = self.box_mlp(lead_boxes)
        real_emb = self.real_mask_mlp(lead_real_mask.unsqueeze(-1))

        lead_tokens = lead_feats + lead_id_emb + layout_emb_exp + box_emb + real_emb

        cls_tok = self.cls_token.expand(B, -1, -1)
        page_tok_seq = (page_tok + layout_emb).unsqueeze(1)

        tokens = torch.cat([cls_tok, page_tok_seq, lead_tokens], dim=1)
        tokens = self.transformer(tokens)
        tokens = self.post_norm(tokens)

        cls_out = tokens[:, 0]
        page_out = tokens[:, 1]
        lead_out = tokens[:, 2:]

        strip_pool = self.pool_idx(lead_out, self.idx_strip)
        ii_pool = self.pool_idx(lead_out, self.idx_ii)
        v1_pool = self.pool_idx(lead_out, self.idx_v1)
        inferior_pool = self.pool_idx(lead_out, self.idx_inferior)
        all_pool = lead_out.mean(dim=1)

        strip_flag = self.strip_flag_mlp(has_rhythm_strip.unsqueeze(1))

        rhythm_fusion = torch.cat([cls_out, page_out, strip_pool, ii_pool, v1_pool, inferior_pool, strip_flag], dim=1)
        atrial_fusion = torch.cat([ii_pool, v1_pool, strip_pool, inferior_pool, strip_flag], dim=1)
        pace_fusion = torch.cat([cls_out, page_out, strip_pool, strip_flag], dim=1)
        binary_fusion = torch.cat([cls_out, page_out, all_pool], dim=1)
        super_fusion = torch.cat([cls_out, page_out, inferior_pool], dim=1)
        layout_fusion = torch.cat([cls_out, page_out], dim=1)

        base_rhythm_logits = self.rhythm_head(rhythm_fusion)

        tiles_flat = x_tiles.reshape(B * T, 3, Ht, Wt)
        tile_feat = self.tile_backbone(tiles_flat)
        tile_feat = self.tile_proj(tile_feat).reshape(B, T, -1)

        tile_logits = self.tile_attn(tile_feat).squeeze(-1)
        tile_attn = torch.softmax(tile_logits.float(), dim=1).to(tile_logits.dtype)
        tile_pool = (tile_feat * tile_attn.unsqueeze(-1)).sum(dim=1)

        residual_in = torch.cat([page_out, tile_pool], dim=1)
        residual_logits = self.context_rhythm_residual(residual_in)

        gate = torch.tanh(self.context_gate)
        rhythm_logits = base_rhythm_logits + gate * residual_logits

        return {
            "rhythm_logits": rhythm_logits,
            "base_rhythm_logits": base_rhythm_logits,
            "residual_rhythm_logits": residual_logits,
            "binary_logits": self.binary_head(binary_fusion),
            "layout_logits": self.layout_head(layout_fusion),
            "rhythm_super_logits": self.rhythm_super_head(super_fusion),
            "atrial_aux_logits": self.atrial_aux_head(atrial_fusion),
            "pace_aux_logits": self.pace_aux_head(pace_fusion).squeeze(1),
        }

def build_model(device=None, n_rhythm=7, n_binary=9, n_layout=5):
    model = ECGV9ContextResidual(
        n_rhythm=n_rhythm,
        n_binary=n_binary,
        n_layout=n_layout,
        d_model=192,
        dropout=0.25
    )
    if device is not None:
        model = model.to(device)
    return model

def unpack_checkpoint_state(state):
    if isinstance(state, dict) and "model_state_dict" in state:
        return state["model_state_dict"]
    if isinstance(state, dict) and "state_dict" in state:
        return state["state_dict"]
    return state

def partial_load_by_shape(model, state_dict):
    model_state = model.state_dict()
    loaded, skipped = [], []

    for k, v in state_dict.items():
        if k in model_state and model_state[k].shape == v.shape:
            model_state[k] = v
            loaded.append(k)
        else:
            skipped.append(k)

    model.load_state_dict(model_state, strict=False)
    return loaded, skipped

def load_checkpoint_flex(model, checkpoint_path, map_location="cpu"):
    state = torch.load(checkpoint_path, map_location=map_location)
    state = unpack_checkpoint_state(state)
    loaded, skipped = partial_load_by_shape(model, state)
    return {"loaded_keys": loaded, "skipped_keys": skipped}
