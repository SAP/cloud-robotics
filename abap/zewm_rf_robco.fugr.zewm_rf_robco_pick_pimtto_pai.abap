FUNCTION zewm_rf_robco_pick_pimtto_pai.
*"----------------------------------------------------------------------
*"*"Local Interface:
*"  CHANGING
*"     REFERENCE(SELECTION) TYPE  /SCWM/S_RF_SELECTION
*"     REFERENCE(RESOURCE) TYPE  /SCWM/S_RSRC
*"     REFERENCE(WHO) TYPE  /SCWM/S_WHO_INT
*"     REFERENCE(ORDIM_CONFIRM) TYPE  /SCWM/S_RF_ORDIM_CONFIRM
*"     REFERENCE(TT_ORDIM_CONFIRM) TYPE  /SCWM/TT_RF_ORDIM_CONFIRM
*"     REFERENCE(TT_NESTED_HU) TYPE  /SCWM/TT_RF_NESTED_HU
*"     REFERENCE(T_RF_PICK_HUS) TYPE  /SCWM/TT_RF_PICK_HUS
*"     REFERENCE(CT_SERNR) TYPE  /SCWM/TT_RF_SERNR
*"     REFERENCE(CT_SERNR_DIFF) TYPE  /SCWM/TT_RF_SERNR
*"     REFERENCE(CS_SN) TYPE  /SCWM/S_RF_SN
*"     REFERENCE(WME_VERIF) TYPE  /SCWM/S_WME_VERIF
*"     REFERENCE(CT_SERNR_LSCK) TYPE  /SCWM/TT_RF_SERNR
*"----------------------------------------------------------------------

 DATA lv_fcode TYPE /scwm/de_fcode.

* call standard
  CALL FUNCTION '/SCWM/RF_PICK_PIMTTO_PAI'
    CHANGING
      selection        = selection
      resource         = resource
      who              = who
      ordim_confirm    = ordim_confirm
      tt_ordim_confirm = tt_ordim_confirm
      tt_nested_hu     = tt_nested_hu
      t_rf_pick_hus    = t_rf_pick_hus
      ct_sernr         = ct_sernr
      ct_sernr_diff    = ct_sernr_diff
      cs_sn            = cs_sn
      wme_verif        = wme_verif
      ct_sernr_lsck    = ct_sernr_lsck.

ENDFUNCTION.
