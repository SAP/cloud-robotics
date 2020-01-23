FUNCTION zewm_rf_robco_0100_pai.
*"----------------------------------------------------------------------
*"*"Local Interface:
*"  CHANGING
*"     REFERENCE(CS_ROBOT) TYPE  ZEWM_S_RF_ROBOT
*"     REFERENCE(SELECTION) TYPE  /SCWM/S_RF_SELECTION
*"     REFERENCE(RESOURCE) TYPE  /SCWM/S_RSRC
*"     REFERENCE(ORDIM_CONFIRM) TYPE  /SCWM/S_RF_ORDIM_CONFIRM
*"     REFERENCE(WHO) TYPE  /SCWM/S_WHO_INT
*"     REFERENCE(T_RF_PICK_HUS) TYPE  /SCWM/TT_RF_PICK_HUS
*"     REFERENCE(TT_ORDIM_CONFIRM) TYPE  /SCWM/TT_RF_ORDIM_CONFIRM
*"     REFERENCE(WME_VERIF) TYPE  /SCWM/S_WME_VERIF
*"----------------------------------------------------------------------

  DATA: lv_who     TYPE /scwm/de_who,
        lv_who_sub TYPE /scwm/de_who.


  BREAK-POINT ID zewm_robco.

* assume background processing
  /scwm/cl_rf_bll_srvc=>set_prmod( /scwm/cl_rf_bll_srvc=>c_prmod_background ).

  IF cs_robot-robot IS INITIAL.
*   batch must be entered
    /scwm/cl_rf_bll_srvc=>set_prmod( /scwm/cl_rf_bll_srvc=>c_prmod_foreground ).
    RETURN.
  ENDIF.

* get open warehouse order for robot resource
  SELECT SINGLE who FROM /scwm/who INTO lv_who
    WHERE ( status  = wmegc_wo_open OR
            status  = wmegc_wo_in_process )
    AND   rsrc = cs_robot-robot.

  IF sy-subrc <> 0 OR lv_who IS INITIAL.
*   No open putway tasks found for batch &1
    MESSAGE e036(zewm_cl_msg) WITH cs_robot-robot.
  ELSE.
*   get sub-whos for who
    SELECT SINGLE who FROM /scwm/who INTO lv_who_sub
      WHERE ( status = wmegc_wo_open OR
              status = wmegc_wo_in_process )
      AND   topwhoid = lv_who.

*   check sub-who found
    IF lv_who_sub IS NOT INITIAL.

*     Simulate logical transaction PIPWHO
      /scwm/cl_rf_bll_srvc=>set_ltrans_simu( 'PIBWHO' ).
      selection-who = lv_who_sub.

*     call standard fm
      CALL FUNCTION '/SCWM/RF_PICK_PIBUSR_PAI'
        CHANGING
          selection        = selection
          resource         = resource
          ordim_confirm    = ordim_confirm
          who              = who
          t_rf_pick_hus    = t_rf_pick_hus
          tt_ordim_confirm = tt_ordim_confirm
          wme_verif        = wme_verif.

    ENDIF.
  ENDIF.

ENDFUNCTION.
