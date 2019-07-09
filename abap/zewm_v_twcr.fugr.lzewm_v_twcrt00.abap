*---------------------------------------------------------------------*
*    view related data declarations
*   generation date: 25.04.2019 at 12:25:47
*   view maintenance generator version: #001407#
*---------------------------------------------------------------------*
*...processing: ZEWM_V_TWCR.....................................*
TABLES: ZEWM_V_TWCR, *ZEWM_V_TWCR. "view work areas
CONTROLS: TCTRL_ZEWM_V_TWCR
TYPE TABLEVIEW USING SCREEN '0001'.
DATA: BEGIN OF STATUS_ZEWM_V_TWCR. "state vector
          INCLUDE STRUCTURE VIMSTATUS.
DATA: END OF STATUS_ZEWM_V_TWCR.
* Table for entries selected to show on screen
DATA: BEGIN OF ZEWM_V_TWCR_EXTRACT OCCURS 0010.
INCLUDE STRUCTURE ZEWM_V_TWCR.
          INCLUDE STRUCTURE VIMFLAGTAB.
DATA: END OF ZEWM_V_TWCR_EXTRACT.
* Table for all entries loaded from database
DATA: BEGIN OF ZEWM_V_TWCR_TOTAL OCCURS 0010.
INCLUDE STRUCTURE ZEWM_V_TWCR.
          INCLUDE STRUCTURE VIMFLAGTAB.
DATA: END OF ZEWM_V_TWCR_TOTAL.

*.........table declarations:.................................*
TABLES: /SCWM/TWCR                     .
